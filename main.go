package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"
  "regexp"

	"github.com/iancoleman/strcase"
	"github.com/stayradiated/deezer"
)

type TapedeckTrack struct {
	Title     string `json:"title"`
	Artist    string `json:"artist"`
	Album     string `json:"album"`
	AlbumArt  string `json:"albumArt,omitempty"`
	AlbumYear int    `json:"albumYear,omitempty"`
	Tiemstamp string `json:"timestamp,omitempty"`
}

type TapedeckPlaylist struct {
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

func filenamify (name string, extension string) string {
  exp := regexp.MustCompile("[^A-z0-9\\-]")
  return exp.ReplaceAllString(strcase.ToKebab(name), "") + extension
}

func convertDate(date string) (year, month, day int) {
	t, _ := time.Parse("2006-01-02", date)
	return t.Year(), int(t.Month()), t.Day()
}

func searchTrack(query string) (*deezer.Track, error) {
	trackList, err := deezer.SearchTrack(query, false, deezer.RANKING, 0, 10)
	if err != nil {
		return nil, err
	}

	fmt.Printf("» Search results for '%s':\n", query)

	for i, track := range trackList {
		trackURL := fmt.Sprintf("https://deezer.com/us/track/%d", track.ID)

		fmt.Printf("%d. %s • %s • %s • %s\n", i, track.Title, track.Artist.Name, track.Album.Title, trackURL)
	}

  if (len(trackList) == 0) {
    return nil, fmt.Errorf("Could not find any tracks...")
  }

	var selectedIndex int

	fmt.Print("Select a track: ")
	fmt.Scanf("%d", &selectedIndex)

	return &trackList[selectedIndex], nil
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

		deezerTrack, err := searchTrack(fmt.Sprintf("%s %s", track.Title, track.Artist))
		if err != nil {
			fmt.Println(err)
      continue
		}

		deezerAlbum, err := deezer.GetAlbum(deezerTrack.Album.ID)
		if err != nil {
			return err
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
