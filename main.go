package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/iancoleman/strcase"
	"github.com/stayradiated/deezer"
)

type TapedeckTrack struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	AlbumArt  string `json:"albumArt,omitempty"`
	AlbumYear int    `json:"albumYear,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

type TapedeckPlaylist struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	CreatedAt string          `json:"createdAt"`
	Audio     string          `json:"audio"`
	Tracks    []TapedeckTrack `json:"tracks"`
}

func downloadFile(srcUrl string, filePath string) error {
	res, err := http.Get(srcUrl)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := io.Copy(file, res.Body); err != nil {
		return err
	}

	return nil
}

func readTapedeckPlaylist(path string) (*TapedeckPlaylist, error) {
	src, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	playlist := TapedeckPlaylist{}
	if err := json.Unmarshal(src, &playlist); err != nil {
		return nil, err
	}

	return &playlist, nil
}

func writeTapedeckPlaylist(playlist *TapedeckPlaylist, path string) error {
	bytes, err := json.Marshal(playlist)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, bytes, 0x644)
	if err != nil {
		return err
	}

	return nil
}

func filenamify(name string, extension string) string {
	exp := regexp.MustCompile("[^A-z0-9\\-]")
	return exp.ReplaceAllString(strcase.ToKebab(name), "") + extension
}

func convertDate(date string) (year, month, day int) {
	t, _ := time.Parse("2006-01-02", date)
	return t.Year(), int(t.Month()), t.Day()
}

func userMenu(trackList deezer.TrackList) (*deezer.Album, error) {
	fmt.Println("0-9: select album")
	fmt.Println("A: enter album ID")
	fmt.Println("?: edit search query")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	userInput := scanner.Text()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if userInput == "A" {
		return userEnterAlbumID()
	}
	if userInput == "?" {
		return userSearchAlbum()
	}

	selectedIndex, err := strconv.Atoi(userInput)
	if err != nil {
		fmt.Println(err)
		return userMenu(trackList)
	}

	if selectedIndex < 0 || selectedIndex >= len(trackList) {
		fmt.Println("Could not find track", selectedIndex)
		return userMenu(trackList)
	}
	track := trackList[selectedIndex]

	album, err := deezer.GetAlbum(track.Album.ID)
	if err != nil {
		return nil, err
	}

	return &album, nil
}

func userSearchAlbum() (*deezer.Album, error) {
	fmt.Print("Enter a query: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	nextQuery := scanner.Text()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	fmt.Println("Next Query", nextQuery)

	if nextQuery == "" {
		fmt.Println("Error: missing search query")
		return userMenu(deezer.TrackList{})
	}

	return searchAlbum(nextQuery)
}

func userEnterAlbumID() (*deezer.Album, error) {
	fmt.Print("Enter an albumID: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	albumIDString := scanner.Text()
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	albumID, err := strconv.Atoi(albumIDString)
	if err != nil {
		fmt.Println("Not a valid number")
		return userMenu(deezer.TrackList{})
	}

	album, err := deezer.GetAlbum(albumID)
	if err != nil {
		return nil, err
	}

	return &album, err
}

func searchAlbum(query string) (*deezer.Album, error) {
	trackList, err := deezer.SearchTrack(query, false, deezer.RANKING, 0, 10)
	if err != nil {
		return nil, err
	}

	fmt.Printf("» Search results for '%s':\n", query)

	for i, track := range trackList {
		trackURL := fmt.Sprintf("https://deezer.com/us/track/%d", track.ID)

		fmt.Printf("%d. %s • %s • %s • %s\n", i, track.Title, track.Artist.Name, track.Album.Title, trackURL)
	}

	if len(trackList) == 0 {
		fmt.Println("↳ No tracks found‥")
	}

	return userMenu(trackList)
}

func printPlaylist(path string) error {
	playlist, err := readTapedeckPlaylist(path)
	if err != nil {
		return err
	}

	fmt.Println(playlist.Name, playlist.CreatedAt)
	for i, track := range playlist.Tracks {
		fmt.Printf("%d. %s • %s • %s\n", i, track.Title, track.Artist, track.Album)
	}

	return nil
}

func autofillPlaylist(path string) error {
	playlist, err := readTapedeckPlaylist(path)
	if err != nil {
		return err
	}

	fmt.Println(playlist.Name, playlist.CreatedAt)
	for i, track := range playlist.Tracks {
		fmt.Printf("Track %d. %s • %s • %s\n", i, track.Title, track.Artist, track.Album)

		if track.Album != "" {
			continue
		}

		deezerAlbum, err := searchAlbum(fmt.Sprintf("%s %s", track.Title, track.Artist))
		if err != nil {
			fmt.Println(err)
			continue
		}

		albumYear, _, _ := convertDate(deezerAlbum.ReleaseDate)

		albumArtPath := filenamify(fmt.Sprintf("%s %s", deezerAlbum.Artist.Name, deezerAlbum.Title), ".jpg")

		albumArtURL := fmt.Sprintf("%s?size=%d", deezerAlbum.Cover, 1000)

		fmt.Printf("Year %d. Album Art: %s\n", albumYear, albumArtPath)

		if err := downloadFile(albumArtURL, albumArtPath); err != nil {
			return err
		}

		playlist.Tracks[i].Album = deezerAlbum.Title
		playlist.Tracks[i].AlbumYear = albumYear
		playlist.Tracks[i].AlbumArt = albumArtPath

		if err := writeTapedeckPlaylist(playlist, path); err != nil {
			return err
		}
	}

	return nil
}

func main() {
	switch os.Args[1] {
	case "print":
		path := os.Args[2]
		if err := printPlaylist(path); err != nil {
			panic(err)
		}

	case "autofill":
		path := os.Args[2]
		if err := autofillPlaylist(path); err != nil {
			panic(err)
		}
	}
}
