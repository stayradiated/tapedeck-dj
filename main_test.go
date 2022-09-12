package main

import (
	"testing"
)

func TestFilenamify(t *testing.T) {
	output := filenamify("Hello/WHAT/ARE/ you /DOING?", ".jpg")

	if output != "hellowhatare-you-doing.jpg" {
		t.Errorf("Filenamify returned '%s'", output)
	}
}
