/*
 * Copyright (c) 2023 Andreas Signer <asigner@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify it
 * under the terms of the GNU General Public License as published by the
 * Free Software Foundation, either version 3 of the License, or (at your
 * option) any later version.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
 * or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General Public License
 * for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program. If not, see <https://www.gnu.org/licenses/>.
 */

package main

import (
	"archive/zip"
	"errors"
	"flag"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/draw"

	_ "image/gif"
	_ "image/jpeg"
	"image/png"
)

/*
                                 640px
┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃                   ╶╮                    ╷                                     ┃
┃                    ├╴65px               │ GAME LIST                           ┃
┃    (15,45)        ╶╯                    │ ...                                 ┃
┃     ┌─────────────────────────────┐     │ ...                                 ┃
┃     │                       ^     │     │ ...                                 ┃
┃     │                       |     │     │ ...                                 ┃
┃     │                       |     │     │ ...                                 ┃
┃╰─┬─╯│                     350px   │╰─┬─╯│ ...                                 ┃  480px
┃ 15px│                       |     │ 15px│ ...                                 ┃
┃     │                       |     │     │ ...                                 ┃
┃     │        <-- 320 px --> v     │     │ ...                                 ┃
┃     └─────────────────────────────┘     │ ...                                 ┃
┃                   ╶╮                    │ ...                                 ┃
┃                    ├╴65px               │ ...                                 ┃
┃                   ╶╯                    ╵                                     ┃
┗━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
                                          ^
                                          ╷
                                          ╰──── 350px

*/

const (
	artworkX    = 15
	artworkY    = 65
	artworkMaxW = 320
	artworkMaxH = 350

	screenW = 640
	screenH = 480
)

var (
	flagRomDir        = flag.String("rom_dir", "", "Root directory of all roms")
	flagMameExtrasDir = flag.String("mame_extras", "", "MAME Extras directory")
	flagMediaDir      = flag.String("media_dir", "media", "")
	flagConsoles      = flag.String("consoles", "gb,gbc,gba,arcade,mame2000", "Consoles to look at")

	logger = log.Default()
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func loadArtwork(mediaDir, mameExtrasDir, console, game string) (image.Image, error) {
	if console == "mame2000" {
		// Try to get it from zip
		archive, err := zip.OpenReader(filepath.Join(mameExtrasDir, "titles.zip"))
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		for _, f := range archive.File {
			if f.FileInfo().IsDir() {
				continue
			}
			filename := f.FileInfo().Name()
			filename = strings.TrimSuffix(filename, filepath.Ext(filename))
			if filename == game {
				r, err := f.Open()
				img, _, err := image.Decode(r)
				archive.Close()
				return img, err
			}
		}
		return nil, errors.New("No artwork found")
	}

	// Check for png, gif, and jpg
	for _, ext := range []string{".png", ".gif", ".jpg"} {
		artWorkFile := filepath.Join(mediaDir, game+ext)
		if fileExists(artWorkFile) {
			f, err := os.Open(artWorkFile)
			if err != nil {
				continue
			}
			defer f.Close()
			image, _, err := image.Decode(f)
			return image, err
		}
	}

	return nil, errors.New("No artwork file found")
}

func scaleImage(img image.Image, w, h int) image.Image {
	scaled := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(scaled, scaled.Rect, img, img.Bounds(), draw.Over, nil)
	return scaled
}

func genImage(mediaDir, mameExtrasDir, console, game string) (image.Image, error) {
	artwork, err := loadArtwork(mediaDir, mameExtrasDir, console, game)
	if err != nil {
		return nil, err
	}
	bounds := artwork.Bounds()
	origW, origH := float32(bounds.Dx()), float32(bounds.Dy())

	ratio := origW / origH
	w := float32(artworkMaxW)
	h := w / ratio
	if h > artworkMaxH {
		h = artworkMaxH
		w = artworkMaxH * ratio
	}

	posX := artworkX + int((artworkMaxW-w)/2)
	posY := artworkY + int((artworkMaxH-h)/2)

	scaled := scaleImage(artwork, int(w), int(h))

	img := image.NewRGBA(image.Rect(0, 0, screenW, screenH))
	draw.Copy(img, image.Point{posX, posY}, scaled, scaled.Bounds(), draw.Over, nil)

	return img, nil
}

func genImages(romDir, mediaDir, mameExtrasDir, console string) error {
	romDir = filepath.Join(romDir, console)
	mediaDir = filepath.Join(mediaDir, console)
	targetDir := filepath.Join(romDir, "imgs")

	os.Mkdir(targetDir, 0755)
	files, err := ioutil.ReadDir(romDir)
	if err != nil {
		return err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filename := file.Name()
		game := strings.TrimSuffix(filename, filepath.Ext(filename))
		img, err := genImage(mediaDir, mameExtrasDir, console, game)
		if err != nil {
			logger.Printf("Can't generate image for %s/%s: %s\n", console, file.Name(), err)
			continue
		}
		targetName := filepath.Join(targetDir, game+".png")
		out, err := os.Create(targetName)
		if err != nil {
			logger.Printf("Can't create image file %s: %s\n", targetName, err)
			continue
		}
		err = png.Encode(out, img)
		if err != nil {
			logger.Printf("Can't encode %s as PNG: %s\n", targetName, err)
			continue
		}
		out.Close()
		logger.Printf("Created image for %s/%s in %s", console, game, targetName)
	}
	return nil
}

func main() {
	flag.Parse()

	if len(*flagRomDir) == 0 {
		fmt.Printf("--rom_dir not set!\n")
		os.Exit(1)
	}

	consoles := strings.Split(*flagConsoles, ",")
	for _, c := range consoles {
		c = strings.TrimSpace(c)
		genImages(*flagRomDir, filepath.Join(*flagRomDir, *flagMediaDir), *flagMameExtrasDir, c)
	}
}
