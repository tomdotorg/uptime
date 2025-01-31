package main

import (
	"errors"
	"log/slog"
	"strings"
)

var titles = []string{
	"'title' by artist",
	"Random Rhythms",
	"'Dance of the Cowards' by Greater Than One",
	"'The Fragility Cycles (\"Gambuh\")' by Ingram Marshall",
	"'Five Drops: I. Animato' by Beyza Yazgan",
	"'\"Reincarnation\" Magnetic Orchestration for Gongs and Bells' by Tatsuya Nakatani",
	"'Chasin' LeRoy' by Corey Wilkes & Abstrakt Pulse",
}

func parse(s string) (title string, artist string, err error) {
	if s[0] != '\'' && len(s) > 1 {
		return s, "", nil
	} else if len(s) == 0 {
		return "", "", errors.New("missing title")
	} else {
		endIndex := strings.Index(s[1:], "' by")
		title = s[1 : endIndex+1]
		artist = s[endIndex+5:]
		return title, artist, nil
	}
}

func main() {
	for _, rawTitle := range titles {
		title, artist, err := parse(rawTitle)
		slog.Info(title + " / " + artist)
		if err != nil {
			return
		}
	}
}
