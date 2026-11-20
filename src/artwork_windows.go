package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"image"
	"image/color"
	"image/draw"

	imagedraw "golang.org/x/image/draw"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"golang.org/x/sys/windows"
)

//go:embed assets/psp.png
var pspPhoto []byte

type artworkPage struct {
	view        Composite
	start       func()
	updateDisc  func(string, string)
	dispose     func()
	stopPreview func()
}

func makeArtworkPage(owner **walk.MainWindow, job func() conversion, selected, defaults *artwork, closed *bool) artworkPage {
	panel := SolidColorBrush{Color: walk.RGB(38, 41, 51)}
	field := SolidColorBrush{Color: walk.RGB(29, 31, 39)}
	ink := walk.RGB(245, 245, 250)
	var preview *walk.ImageView
	var status, discSummary *walk.Label
	var serial *walk.LineEdit
	var bitmap *walk.Bitmap
	var controls []*walk.PushButton
	var playMotion *walk.PushButton
	var playbackCancel context.CancelFunc
	var playbackRun int
	var animationFrame image.Image
	artworkBusy := false
	ctx, cancel := context.WithCancel(context.Background())
	bundledPhoto, _ := readArtwork(bytes.NewReader(pspPhoto))
	photo := bundledPhoto
	frameDir := filepath.Dir(filepath.Dir(job().engine))
	if custom, err := readLocalArtwork(filepath.Join(frameDir, "preview-frame.png")); err == nil &&
		custom.Bounds().Size() == (image.Point{X: 720, Y: 504}) {
		photo = custom
	}

	resourceDir := filepath.Join(filepath.Dir(job().engine), "Resources")
	defaultIcon, _ := readLocalArtwork(filepath.Join(resourceDir, "ICON0.PNG"))
	defaultBackground, _ := readLocalArtwork(filepath.Join(resourceDir, "PIC1.PNG"))
	backdrop := color.NRGBA{R: 20, G: 22, B: 32, A: 255}
	fillPreview := func() {
		bg := selected.background
		if bg == nil {
			bg = defaults.background
			if bg == nil {
				bg = fitArtwork(defaultBackground, backgroundBounds, backdrop)
			}
		}
		canvas := image.NewNRGBA(backgroundBounds)
		draw.Draw(canvas, canvas.Bounds(), bg, bg.Bounds().Min, draw.Src)
		icon := animationFrame
		if icon == nil {
			icon = selected.icon
		}
		if icon == nil {
			icon = defaults.icon
			if icon == nil {
				icon = fitArtwork(defaultIcon, iconBounds, color.Transparent)
			}
		}
		draw.Draw(canvas, iconBounds.Add(image.Pt(24, 96)), icon, icon.Bounds().Min, draw.Over)
		var previewImage image.Image = canvas
		if photo != nil {
			previewImage = frontPSPPreview(photo, canvas)
		}
		next, err := walk.NewBitmapFromImage(previewImage)
		if err == nil {
			err = preview.SetImage(next)
			if err != nil {
				next.Dispose()
			}
		}
		if err != nil {
			status.SetText("Could not draw the artwork preview.")
			return
		}
		if bitmap != nil {
			bitmap.Dispose()
		}
		bitmap = next
	}

	setBusy := func(value bool) {
		artworkBusy = value
		playMotion.SetEnabled(!value && len(selected.motion) != 0)
		serial.SetEnabled(!value)
		for _, control := range controls {
			control.SetEnabled(!value)
		}
	}

	stopPreview := func() {
		if playbackCancel == nil {
			return
		}
		playbackRun++
		playbackCancel()
		playbackCancel = nil
		animationFrame = nil
		playMotion.SetText("Play PMF")
		fillPreview()
		status.SetText("PMF preview stopped.")
	}
	setMotion := func(data []byte) {
		stopPreview()
		selected.motion = data
		playMotion.SetEnabled(!artworkBusy && len(data) != 0)
	}

	setImage := func(target *image.Image, img image.Image, background bool) {
		if background {
			*target = fitArtwork(img, backgroundBounds, backdrop)
		} else {
			*target = fitArtwork(img, iconBounds, color.Transparent)
		}
		fillPreview()
	}

	var detect, chooseMotion, clearMotion, chooseFrame *walk.PushButton
	var motionLabel *walk.Label
	var scan, createMotion *walk.PushButton
	var scenePicker *walk.ComboBox
	var scenes []cutscene
	var scenePath string
	toolsDir := filepath.Join(filepath.Dir(filepath.Dir(job().engine)), "Tools")
	startHidden := func(cmd *exec.Cmd) {
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	}

	playPreview := func() {
		if playbackCancel != nil {
			stopPreview()
			return
		}
		if len(selected.motion) == 0 {
			return
		}
		playback, stop := context.WithCancel(ctx)
		playbackCancel = stop
		playbackRun++
		run := playbackRun
		motion := selected.motion
		playMotion.SetText("Stop preview")
		status.SetText("Decoding PMF preview…")
		go func() {
			decodeCtx, stopDecoding := context.WithTimeout(playback, 90*time.Second)
			ffmpeg, err := prepareFFmpeg(decodeCtx, toolsDir)
			var frames []byte
			if err == nil {
				frames, err = decodePMFPreview(decodeCtx, ffmpeg, motion, startHidden)
			}
			stopDecoding()
			if err != nil {
				(*owner).Synchronize(func() {
					if *closed || run != playbackRun {
						return
					}
					stopPreview()
					status.SetText("Could not play this PMF. Check that it contains a PSP video stream.")
				})
				return
			}

			ticker := time.NewTicker(time.Second / pmfPreviewFPS)
			defer ticker.Stop()
			frameCount := len(frames) / pmfFrameSize()
			started := time.Now()
			for {
				index := int(time.Since(started)*pmfPreviewFPS/time.Second) % frameCount
				painted := make(chan struct{})
				(*owner).Synchronize(func() {
					defer close(painted)
					if *closed || run != playbackRun {
						return
					}
					animationFrame = pmfPreviewFrame(frames, index)
					if index == 0 {
						status.SetText("Playing silent PMF preview.")
					}
					fillPreview()
				})
				select {
				case <-playback.Done():
					return
				case <-painted:
				}
				select {
				case <-playback.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}

	row := func(title string, target, defaultImage *image.Image, background bool, buttons []*walk.PushButton) Widget {
		return Composite{Background: panel, Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
			Label{TextColor: ink, Text: title},
			HSpacer{},
			PushButton{AssignTo: &buttons[0], Text: "Choose image…", MinSize: Size{Width: 110, Height: 31}, OnClicked: func() {
				file := walk.FileDialog{Title: "Choose artwork", Filter: "Images (*.png;*.jpg;*.jpeg)|*.png;*.jpg;*.jpeg"}
				ok, err := file.ShowOpen(*owner)
				if err != nil {
					status.SetText(err.Error())
					return
				}
				if !ok {
					return
				}
				img, err := readLocalArtwork(file.FilePath)
				if err != nil {
					status.SetText(err.Error())
					return
				}
				status.SetText(filepath.Base(file.FilePath))
				setImage(target, img, background)
			}},
			PushButton{AssignTo: &buttons[1], Text: "Fetch cover", MinSize: Size{Width: 94, Height: 31}, OnClicked: func() {
				id := serial.Text()
				path := job().input
				status.SetText("Looking up cover…")
				setBusy(true)
				go func() {
					img, err := fetchCover(ctx, id)
					(*owner).Synchronize(func() {
						if *closed {
							return
						}
						setBusy(false)
						if path != job().input {
							status.SetText("Disc changed. Fetch artwork for the new disc.")
							return
						}
						if err != nil {
							status.SetText(err.Error())
							return
						}
						status.SetText("Loaded online cover for " + normalizeSerial(id))
						setImage(target, img, background)
					})
				}()
			}},
			PushButton{AssignTo: &buttons[2], Text: "Use default", MinSize: Size{Width: 94, Height: 31}, OnClicked: func() {
				*target = nil
				status.SetText("Restored default artwork.")
				fillPreview()
			}},
			PushButton{AssignTo: &buttons[3], Text: "Save as default", MinSize: Size{Width: 108, Height: 31}, OnClicked: func() {
				if *target == nil {
					status.SetText("Choose an image first.")
					return
				}
				name := "ICON0.png"
				if background {
					name = "PIC1.png"
				}
				dir := filepath.Join(filepath.Dir(toolsDir), "Artwork Defaults")
				if err := saveDefaultArtwork(dir, name, *target); err != nil {
					status.SetText(err.Error())
					return
				}
				*defaultImage = *target
				status.SetText("Saved as the default for future launches.")
			}},
		}}
	}

	iconButtons, backgroundButtons := make([]*walk.PushButton, 4), make([]*walk.PushButton, 4)
	view := Composite{
		Background: panel, StretchFactor: 1, Visible: false,
		Layout: VBox{Margins: Margins{Left: 20, Top: 16, Right: 20, Bottom: 16}, Spacing: 9},
		Children: []Widget{
			Composite{Background: panel, Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
				Label{TextColor: ink, Text: "PSP preview", Font: Font{Family: "Segoe UI", PointSize: 15, Bold: true}},
				HSpacer{},
				PushButton{AssignTo: &chooseFrame, Text: "Choose preview frame…", MinSize: Size{Width: 165}, OnClicked: func() {
					dialog := walk.FileDialog{Title: "Choose a 720 × 504 PSP frame", Filter: "PNG image (*.png)|*.png"}
					ok, err := dialog.ShowOpen(*owner)
					if err != nil {
						status.SetText(err.Error())
						return
					}
					if !ok {
						return
					}
					img, err := readLocalArtwork(dialog.FilePath)
					if err != nil {
						status.SetText(err.Error())
						return
					}
					if img.Bounds().Size() != (image.Point{X: 720, Y: 504}) {
						status.SetText("Choose a 720 × 504 PNG with the same screen placement as the supplied frame.")
						return
					}
					if err := saveDefaultArtwork(frameDir, "preview-frame.png", img); err != nil {
						status.SetText(err.Error())
						return
					}
					photo = img
					fillPreview()
					status.SetText("Saved the custom PSP preview frame.")
				}},
			}},
			ImageView{AssignTo: &preview, Mode: ImageViewModeShrink, MinSize: Size{Height: 155}, MaxSize: Size{Height: 282}},
			Label{AssignTo: &discSummary, TextColor: ink, Text: "No disc selected", TextAlignment: AlignCenter},
			row("Icon · 144 × 80 px", &selected.icon, &defaults.icon, false, iconButtons),
			row("Background · 480 × 272 px", &selected.background, &defaults.background, true, backgroundButtons),
			Composite{Background: panel, Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
				Label{TextColor: ink, Text: "PS1 game ID"},
				LineEdit{AssignTo: &serial, Background: field, TextColor: ink, CueBanner: "Example: SLUS-00067"},
				PushButton{AssignTo: &detect, Text: "Detect from disc", OnClicked: func() {
					currentJob := job()
					if strings.TrimSpace(currentJob.input) == "" {
						status.SetText("Choose a disc image in the main window or enter its game ID.")
						return
					}
					status.SetText("Reading game ID…")
					setBusy(true)
					go func() {
						detectCtx, stop := context.WithTimeout(ctx, 90*time.Second)
						defer stop()
						id, err := currentJob.detectSerial(detectCtx, func(cmd *exec.Cmd) {
							cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
						})
						(*owner).Synchronize(func() {
							if *closed {
								return
							}
							setBusy(false)
							if currentJob.input != job().input {
								return
							}
							if err != nil {
								status.SetText("Could not detect the game ID. Enter it manually.")
								return
							}
							serial.SetText(id)
							status.SetText(fmt.Sprintf("Detected %s. Choose which artwork to fetch.", id))
						})
					}()
				}},
			}},
			Composite{Background: panel, Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
				Label{TextColor: ink, AssignTo: &motionLabel, Text: "PSP menu animation · ICON1.PMF"},
				HSpacer{},
				PushButton{AssignTo: &chooseMotion, Text: "Choose PMF…", OnClicked: func() {
					file := walk.FileDialog{Title: "Choose PSP menu animation", Filter: "PSP movie (*.pmf)|*.pmf"}
					if ok, err := file.ShowOpen(*owner); err != nil {
						status.SetText(err.Error())
					} else if ok {
						data, err := readMotion(file.FilePath)
						if err != nil {
							status.SetText(err.Error())
							return
						}
						setMotion(data)
						motionLabel.SetText("PMF animation attached")
						status.SetText("Click Play PMF to preview the attached animation.")
					}
				}},
				PushButton{AssignTo: &playMotion, Text: "Play PMF", MinSize: Size{Width: 105}, Enabled: false, OnClicked: playPreview},
				PushButton{AssignTo: &clearMotion, Text: "Remove", OnClicked: func() {
					setMotion(nil)
					motionLabel.SetText("PSP menu animation · ICON1.PMF")
					status.SetText("Removed the menu animation.")
				}},
			}},
			Composite{Background: panel, Layout: HBox{MarginsZero: true, Spacing: 8}, Children: []Widget{
				Label{TextColor: ink, Text: "Disc cutscenes"},
				PushButton{AssignTo: &scan, Text: "Find cutscenes", MinSize: Size{Width: 112}, OnClicked: func() {
					path := job().input
					if path == "" {
						status.SetText("Drop a raw BIN or CUE first.")
						return
					}
					status.SetText("Scanning the disc for STR videos…")
					setBusy(true)
					go func() {
						found, err := scanCutscenes(ctx, path)
						(*owner).Synchronize(func() {
							if *closed {
								return
							}
							setBusy(false)
							if job().input != path {
								return
							}
							if err != nil {
								status.SetText(err.Error())
								return
							}
							scenes, scenePath = found, path
							labels := make([]string, len(found))
							for i, s := range found {
								labels[i] = fmt.Sprintf("Clip %d · %d × %d · %d frames", i+1, s.width, s.height, s.frames)
							}
							scenePicker.SetModel(labels)
							scenePicker.SetCurrentIndex(0)
							status.SetText("Clips are sorted by length. Choose one to make a five-second menu animation.")
						})
					}()
				}},
				ComboBox{AssignTo: &scenePicker, Model: []string{}, MinSize: Size{Width: 175}},
				PushButton{AssignTo: &createMotion, Text: "Make PMF", MinSize: Size{Width: 94}, OnClicked: func() {
					index := scenePicker.CurrentIndex()
					if scenePath != job().input || index < 0 || index >= len(scenes) {
						status.SetText("Find and select a cutscene first.")
						return
					}
					scene, path := scenes[index], scenePath
					status.SetText("Preparing bundled FFmpeg and encoding the cutscene…")
					setBusy(true)
					go func() {
						workCtx, stop := context.WithTimeout(ctx, 3*time.Minute)
						defer stop()
						encoder, err := prepareFFmpeg(workCtx, toolsDir)
						var pmf []byte
						if err == nil {
							pmf, err = makeCutscenePMF(workCtx, scene, toolsDir, encoder, filepath.Join(toolsDir, "Python", "python.exe"), startHidden)
						}
						(*owner).Synchronize(func() {
							if *closed {
								return
							}
							setBusy(false)
							if job().input != path {
								return
							}
							if err != nil {
								message := strings.SplitN(err.Error(), "\n", 2)[0]
								if len(message) > 125 {
									message = message[:125]
								}
								status.SetText(message)
								return
							}
							setMotion(pmf)
							motionLabel.SetText("Cutscene PMF attached")
							status.SetText("The next EBOOT.PBP export will include this animation.")
						})
					}()
				}},
			}},
			Label{TextColor: ink, Text: "Online covers from xlenore/psx-covers. Local artwork supports PNG and JPEG."},
			Label{TextColor: ink, AssignTo: &status, Text: "Artwork changes apply to the next export."},
		},
	}
	return artworkPage{
		view:        view,
		stopPreview: stopPreview,
		start: func() {
			controls = append(append(iconButtons, backgroundButtons...), detect, chooseMotion, clearMotion, scan, createMotion, chooseFrame)
			themeButton(playMotion, panel.Color, true)
			for _, button := range controls {
				themeButton(button, panel.Color, false)
			}
			fillPreview()
		},
		updateDisc: func(title, id string) {
			if scenePath != job().input {
				scenes, scenePath = nil, ""
				scenePicker.SetModel([]string{})
			}
			serial.SetText(id)
			if title == "" {
				title = "No disc selected"
			}
			if id != "" {
				title += " · " + id
			}
			discSummary.SetText(title)
		},
		dispose: func() {
			cancel()
			if bitmap != nil {
				bitmap.Dispose()
			}
		},
	}
}

func frontPSPPreview(photo, screen image.Image) image.Image {
	const left, top, right, bottom = 8, 106, 711, 416
	bounds := image.Rect(0, 0, right-left, bottom-top)
	frame := image.NewNRGBA(bounds)
	draw.Draw(frame, bounds, photo, image.Pt(left, top), draw.Src)
	imagedraw.CatmullRom.Scale(frame, image.Rect(158-left, 135-top, 562-left, 363-top), screen, screen.Bounds(), draw.Src, nil)
	return frame
}
