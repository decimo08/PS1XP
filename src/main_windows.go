package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
	"golang.org/x/sys/windows"
)

func main() {
	if err := showWindow(); err != nil {
		walk.MsgBox(nil, "PS1 to EBOOT", err.Error(), walk.MsgBoxIconError)
	}
}

func showWindow() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	root := filepath.Dir(exe)
	engine := filepath.Join(root, "Engine", "psxpackager.exe")
	if _, err := os.Stat(engine); err != nil {
		return errors.New("PSXPackager is missing. Extract the entire ZIP and run the app from the extracted folder")
	}

	var window *walk.MainWindow
	var destination, detailsSerial *walk.LineEdit
	var inputPath string
	var requestNumber int
	var closing bool
	var discName, discDetails, detailsTitle, detailsFormat *walk.Label
	var convertPage, artworkContainer, conversionFooter *walk.Composite
	var discRow, idleDrop, selectedFill *walk.Composite
	var dropTitle, dropHint, dropIcon *walk.Label
	var navConvert, navArtwork, navSettings, removeDisc, logButton *walk.PushButton
	var level *walk.ComboBox
	var browseOutput, convert, cancelButton, openOutput *walk.PushButton
	var savedDir string
	var selectedArt artwork
	defaultDir := filepath.Join(root, "Artwork Defaults")
	defaultIcon, _ := readLocalArtwork(filepath.Join(defaultDir, "ICON0.png"))
	defaultBackground, _ := readLocalArtwork(filepath.Join(defaultDir, "PIC1.png"))
	defaultArt := artwork{icon: defaultIcon, background: defaultBackground}
	var progress *walk.CustomWidget
	progressAmount := 0
	progressLabel := "Preparing disc"
	updateProgress := func(percent int, label string) {
		progressAmount, progressLabel = percent, label
		_ = progress.Invalidate()
	}
	var dropZone *walk.Composite
	var status *walk.Label
	var log string
	var cancel context.CancelFunc
	artPage := makeArtworkPage(&window, func() conversion {
		return conversion{engine: engine, runtime: filepath.Join(root, "Runtime"), input: inputPath}
	}, &selectedArt, &defaultArt, &closing)
	defer artPage.dispose()
	artPage.view.AssignTo = &artworkContainer

	setBusy := func(busy bool) {
		for _, widget := range []walk.Widget{destination, detailsSerial, level, browseOutput, convert, navArtwork} {
			widget.SetEnabled(!busy)
		}
		cancelButton.SetEnabled(busy)
		cancelButton.SetVisible(busy)
		removeDisc.SetEnabled(!busy && inputPath != "")
		openOutput.SetEnabled(!busy && savedDir != "")
	}

	loadImage := func(path string) {
		progress.SetVisible(false)
		inputPath = path
		requestNumber++
		request := requestNumber
		discName.SetText(filepath.Base(path))
		detailsTitle.SetText(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		artPage.updateDisc(detailsTitle.Text(), "")
		kind := strings.ToUpper(strings.TrimPrefix(filepath.Ext(path), "."))
		discDetails.SetText(kind + " · Reading game ID…")
		detailsFormat.SetText(kind)
		detailsSerial.SetText("")
		detailsSerial.SetCueBanner("Reading game ID…")
		detailsSerial.SetEnabled(false)
		idleDrop.SetVisible(false)
		discRow.SetVisible(true)
		selectedFill.SetVisible(true)
		removeDisc.SetVisible(true)
		removeDisc.SetEnabled(true)
		go func() {
			detectJob := conversion{engine: engine, runtime: filepath.Join(root, "Runtime"), input: path}
			detectCtx, done := context.WithTimeout(context.Background(), 90*time.Second)
			defer done()
			id, _ := detectJob.detectSerial(detectCtx, func(cmd *exec.Cmd) {
				cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
			})
			title := titleForSerial(filepath.Join(filepath.Dir(engine), "Resources", "gameInfo.db"), id)
			window.Synchronize(func() {
				if closing || request != requestNumber || inputPath != path {
					return
				}
				if title == "" {
					title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
				}
				artPage.updateDisc(title, id)
				detailsSerial.SetEnabled(true)
				detailsSerial.SetCueBanner("Enter game ID, e.g. SLUS-00627")
				if id == "" {
					discDetails.SetText(kind + " · Game ID unavailable")
				} else {
					discDetails.SetText(kind + " · " + id)
					detailsSerial.SetText(id)
					if title != "" {
						detailsTitle.SetText(title)
					}
				}
			})
		}()
		if destination.Text() == "" {
			destination.SetText(filepath.Dir(path))
		}
		status.SetText("Ready to convert")
	}

	dropImages := func(paths []string) {
		if cancel != nil {
			return
		}
		var cues, images []string
		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil || !info.Mode().IsRegular() {
				continue
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".cue":
				cues = append(cues, path)
			case ".bin", ".iso":
				images = append(images, path)
			}
		}
		switch {
		case len(cues) == 1:
			loadImage(cues[0])
		case len(cues) == 0 && len(images) == 1:
			loadImage(images[0])
		case len(cues) == 0 && len(images) == 0:
			status.SetText("Drop a BIN, CUE or ISO file.")
		default:
			status.SetText("Drop one disc image at a time.")
		}
	}

	run := func() {
		inputPath, err := filepath.Abs(inputPath)
		if err != nil {
			status.SetText("Enter a valid disc image path.")
			return
		}
		info, err := os.Stat(inputPath)
		ext := strings.ToLower(filepath.Ext(inputPath))
		if err != nil || !info.Mode().IsRegular() || (ext != ".bin" && ext != ".cue" && ext != ".iso") {
			status.SetText("Choose an existing BIN, CUE or ISO file.")
			return
		}
		outputPath := strings.TrimSpace(destination.Text())
		if outputPath == "" {
			status.SetText("Choose a folder for EBOOT.PBP.")
			return
		}
		outputPath, err = filepath.Abs(outputPath)
		if err != nil {
			status.SetText("Enter a valid output folder.")
			return
		}
		info, err = os.Stat(outputPath)
		if err != nil || !info.IsDir() {
			status.SetText("Choose an existing output folder.")
			return
		}

		id := normalizeSerial(detailsSerial.Text())
		gameFolder, err := gameOutputFolder(outputPath, id)
		if err != nil {
			status.SetText("Wait for game ID detection or enter the ID in Disc details.")
			return
		}
		if info, err := os.Lstat(gameFolder); err == nil && !info.IsDir() {
			status.SetText("Another item already uses the game ID folder name.")
			return
		} else if err != nil && !os.IsNotExist(err) {
			status.SetText("Could not check the game ID folder.")
			return
		}
		outputFile := filepath.Join(gameFolder, "EBOOT.PBP")
		replace := false
		if target, err := os.Lstat(outputFile); err == nil {
			if !target.Mode().IsRegular() {
				status.SetText("EBOOT.PBP is already used by another item in this folder.")
				return
			}
			msg := "Replace " + id + "\\EBOOT.PBP after the new file is ready?"
			if walk.MsgBox(window, "Replace EBOOT.PBP", msg, walk.MsgBoxYesNo|walk.MsgBoxIconWarning|walk.MsgBoxDefButton2) != walk.DlgCmdYes {
				return
			}
			replace = true
		} else if !os.IsNotExist(err) {
			status.SetText("Could not check for an existing EBOOT.PBP.")
			return
		}

		ctx, stop := context.WithCancel(context.Background())
		cancel = stop
		effectiveArt := selectedArt
		if effectiveArt.icon == nil {
			effectiveArt.icon = defaultArt.icon
		}
		if effectiveArt.background == nil {
			effectiveArt.background = defaultArt.background
		}
		job := conversion{engine: engine, runtime: filepath.Join(root, "Runtime"),
			input: inputPath, outputDir: outputPath, gameID: id, level: []int{0, 5, 9}[level.CurrentIndex()], art: effectiveArt}
		savedDir = ""
		log = ""
		status.SetText("Converting disc image…")
		job.report = func(percent int, label string) {
			window.Synchronize(func() {
				if ctx.Err() == nil {
					updateProgress(percent, label)
				}
			})
		}
		updateProgress(0, "Preparing disc")
		progress.SetVisible(true)
		setBusy(true)
		go func() {
			defer stop()
			details, result := job.run(ctx, func(cmd *exec.Cmd) {
				cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
			}, func(source, target string) error {
				sourcePath, err := windows.UTF16PtrFromString(source)
				if err != nil {
					return err
				}
				targetPath, err := windows.UTF16PtrFromString(target)
				if err != nil {
					return err
				}
				var flags uint32
				if replace {
					flags = windows.MOVEFILE_REPLACE_EXISTING
				}
				return windows.MoveFileEx(sourcePath, targetPath, flags)
			})
			window.Synchronize(func() {
				cancel = nil
				switch {
				case errors.Is(result, context.Canceled):
					status.SetText("Conversion canceled.")
					updateProgress(progressAmount, "Canceled")
				case result != nil:
					status.SetText("Conversion failed. See conversion log.")
					updateProgress(progressAmount, "Conversion failed")
					details = fmt.Sprintf("%s\r\n\r\n%s", result, details)
				default:
					savedDir = gameFolder
					status.SetText("Saved " + id + "\\EBOOT.PBP")
					updateProgress(100, "EBOOT.PBP saved")
				}
				setBusy(false)
				log = strings.ReplaceAll(strings.ReplaceAll(details, "\r\n", "\n"), "\n", "\r\n")
			})
		}()
	}

	background := SolidColorBrush{Color: walk.RGB(29, 31, 39)}
	sidebar := SolidColorBrush{Color: walk.RGB(23, 25, 33)}
	panel := SolidColorBrush{Color: walk.RGB(37, 40, 53)}
	dropSurface := SolidColorBrush{Color: walk.RGB(46, 39, 69)}
	accentInk := walk.RGB(204, 165, 255)
	field := SolidColorBrush{Color: walk.RGB(49, 52, 63)}
	ink := walk.RGB(245, 245, 250)
	muted := walk.RGB(191, 194, 208)
	pad := Margins{Left: 15, Top: 13, Right: 15, Bottom: 13}
	font := Font{Family: "Segoe UI", PointSize: 10}
	heading := Font{Family: "Segoe UI", PointSize: 15, Bold: true}
	barBrush, err := walk.NewSolidColorBrush(walk.RGB(42, 45, 59))
	if err != nil {
		return err
	}
	defer barBrush.Dispose()
	barFont, err := walk.NewFont("Segoe UI", 10, walk.FontBold)
	if err != nil {
		return err
	}
	defer barFont.Dispose()

	switchPage := func(showArtwork bool) {
		if !showArtwork {
			artPage.stopPreview()
		}
		convertPage.SetVisible(!showArtwork)
		conversionFooter.SetVisible(!showArtwork)
		artworkContainer.SetVisible(showArtwork)
		setButtonPrimary(navConvert, !showArtwork)
		setButtonPrimary(navArtwork, showArtwork)
	}

	icon, _ := walk.NewIconFromResourceId(2)
	if icon != nil {
		defer icon.Dispose()
	}
	err = (MainWindow{
		AssignTo: &window, Title: "PS1 to EBOOT", Icon: icon, OnDropFiles: dropImages,
		Size: Size{Width: 1200, Height: 750}, MinSize: Size{Width: 865, Height: 535},
		Background: background, Font: font, Layout: VBox{MarginsZero: true, SpacingZero: true},
		Children: []Widget{
			Composite{StretchFactor: 1, Background: background, Layout: HBox{MarginsZero: true, SpacingZero: true}, Children: []Widget{
				Composite{Background: sidebar, MinSize: Size{Width: 168}, MaxSize: Size{Width: 168}, Layout: VBox{Margins: pad, Spacing: 10}, Children: []Widget{
					PushButton{AssignTo: &navConvert, Text: "Convert", MinSize: Size{Width: 110, Height: 38}, OnClicked: func() { switchPage(false) }},
					PushButton{AssignTo: &navArtwork, Text: "Artwork", MinSize: Size{Height: 35}, OnClicked: func() { switchPage(true) }},
					PushButton{AssignTo: &navSettings, Text: "Settings", MinSize: Size{Height: 35}, OnClicked: func() {
						walk.MsgBox(window, "Compression settings", "Balanced (5) is the default and usually finishes faster.\n\nMaximum (9) may make a smaller PBP and can take longer.\n\nBoth preserve the same disc data.", walk.MsgBoxIconInformation)
						switchPage(false)
						level.SetFocus()
					}},
					VSpacer{},
				}},
				Composite{AssignTo: &convertPage, StretchFactor: 1, Background: background, Layout: VBox{Margins: pad, Spacing: 12}, Children: []Widget{
					Label{Text: "Convert PS1", TextColor: ink, Font: Font{Family: "Segoe UI", PointSize: 24, Bold: true}},
					Composite{StretchFactor: 1, Layout: HBox{MarginsZero: true, Spacing: 12}, Children: []Widget{
						Composite{StretchFactor: 2, Layout: VBox{MarginsZero: true, Spacing: 11}, Children: []Widget{
							Composite{AssignTo: &dropZone, Background: panel, StretchFactor: 1, Layout: VBox{Margins: pad, Spacing: 12}, Children: []Widget{
								Composite{Background: panel, Layout: HBox{MarginsZero: true}, Children: []Widget{
									Label{Text: "Disc images", TextColor: ink, Font: heading},
									HSpacer{},
								}},
								Composite{AssignTo: &idleDrop, Background: dropSurface, StretchFactor: 1, MinSize: Size{Height: 175}, Layout: VBox{Margins: pad, Spacing: 8}, Children: []Widget{
									VSpacer{},
									Label{AssignTo: &dropIcon, Text: "◉", TextColor: accentInk, TextAlignment: AlignCenter, Font: Font{Family: "Segoe UI Symbol", PointSize: 29}},
									Label{AssignTo: &dropTitle, Text: "Drop a BIN, CUE or ISO here", TextColor: ink, TextAlignment: AlignCenter, Font: Font{Family: "Segoe UI", PointSize: 18, Bold: true}},
									Label{AssignTo: &dropHint, Text: "For multi-track discs, drop the CUE sheet.", TextColor: muted, TextAlignment: AlignCenter},
									VSpacer{},
								}},
								Composite{AssignTo: &discRow, Visible: false, Background: field, Layout: HBox{Margins: pad, Spacing: 12}, Children: []Widget{
									Label{Text: "◉", TextColor: ink, Font: Font{Family: "Segoe UI Symbol", PointSize: 20}},
									Composite{StretchFactor: 1, Background: field, Layout: VBox{MarginsZero: true, Spacing: 3}, Children: []Widget{
										Label{AssignTo: &discName, TextColor: ink, Font: Font{Family: "Segoe UI", PointSize: 13, Bold: true}},
										Label{AssignTo: &discDetails, TextColor: muted},
									}},
									PushButton{AssignTo: &removeDisc, Text: "×", ToolTipText: "Remove disc", Visible: false, Enabled: false, MaxSize: Size{Width: 38}, OnClicked: func() {
										inputPath = ""
										artPage.updateDisc("", "")
										discRow.SetVisible(false)
										selectedFill.SetVisible(false)
										idleDrop.SetVisible(true)
										detailsTitle.SetText("No disc selected")
										detailsFormat.SetText("—")
										detailsSerial.SetText("")
										detailsSerial.SetCueBanner("Game ID")
										detailsSerial.SetEnabled(false)
										status.SetText("Choose a disc image")
										removeDisc.SetEnabled(false)
										removeDisc.SetVisible(false)
									}},
								}},
								Composite{AssignTo: &selectedFill, Visible: false, Background: panel, StretchFactor: 1, Layout: VBox{MarginsZero: true}, Children: []Widget{VSpacer{}}},
							}},
							Composite{Background: background, Layout: Grid{Columns: 3, MarginsZero: true, Spacing: 9}, Children: []Widget{
								Label{Text: "Save folder", TextColor: ink},
								LineEdit{AssignTo: &destination, CueBanner: "Parent folder for game ID folders", Background: field, TextColor: ink, MinSize: Size{Height: 32}},
								PushButton{AssignTo: &browseOutput, Text: "Browse", MinSize: Size{Width: 85, Height: 32}, OnClicked: func() {
									dialog := walk.FileDialog{Title: "Choose output folder", FilePath: destination.Text()}
									if ok, err := dialog.ShowBrowseFolder(window); err != nil {
										status.SetText(err.Error())
									} else if ok {
										destination.SetText(dialog.FilePath)
									}
								}},
								Label{Text: "Compression", TextColor: ink},
								ComboBox{AssignTo: &level, Model: []string{"None (0)", "Balanced (5)", "Maximum (9)"}, CurrentIndex: 1},
							}},
						}},
						Composite{Background: panel, MinSize: Size{Width: 240}, MaxSize: Size{Width: 290}, Layout: VBox{Margins: pad, Spacing: 10}, Children: []Widget{
							Label{Text: "Disc details", TextColor: ink, Font: heading},
							Label{Text: "Game", TextColor: muted},
							Label{AssignTo: &detailsTitle, Text: "No disc selected", TextColor: ink, Font: Font{Family: "Segoe UI", PointSize: 13, Bold: true}},
							Label{Text: "Image format", TextColor: muted},
							Label{AssignTo: &detailsFormat, Text: "—", TextColor: ink},
							Label{Text: "PS1 game ID", TextColor: muted},
							LineEdit{AssignTo: &detailsSerial, Enabled: false, CueBanner: "Game ID", Background: field, TextColor: ink},
							VSpacer{},
						}},
					}},
				}},
				artPage.view,
			}},
			CustomWidget{
				AssignTo: &progress, Visible: false, MinSize: Size{Height: 30}, MaxSize: Size{Height: 30},
				InvalidatesOnResize: true, PaintMode: PaintBuffered,
				PaintPixels: func(canvas *walk.Canvas, _ walk.Rectangle) error {
					bounds := progress.ClientBoundsPixels()
					if err := canvas.FillRectanglePixels(barBrush, bounds); err != nil {
						return err
					}
					fill := bounds
					fill.Width = bounds.Width * progressAmount / 100
					if fill.Width > 0 {
						if err := canvas.GradientFillRectanglePixels(walk.RGB(103, 51, 173), walk.RGB(132, 61, 205), walk.Horizontal, fill); err != nil {
							return err
						}
					}
					text := fmt.Sprintf("%s  ·  %d%%", progressLabel, progressAmount)
					return canvas.DrawTextPixels(text, barFont, ink, bounds, walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
				},
			},
			Composite{AssignTo: &conversionFooter, Background: panel, Layout: HBox{Margins: pad, Spacing: 8}, Children: []Widget{
				Label{AssignTo: &status, Text: "Choose a disc image", TextColor: muted},
				HSpacer{},
				PushButton{AssignTo: &logButton, Text: "Conversion log", OnClicked: func() {
					var dialog *walk.Dialog
					_, _ = (Dialog{AssignTo: &dialog, Title: "Conversion log", MinSize: Size{Width: 620, Height: 340},
						Layout: VBox{}, Children: []Widget{
							TextEdit{Text: log, ReadOnly: true, VScroll: true},
							PushButton{Text: "Close", OnClicked: func() { dialog.Accept() }},
						}}).Run(window)
				}},
				PushButton{AssignTo: &openOutput, Text: "Open output folder", Enabled: false, OnClicked: func() {
					cmd := exec.Command("explorer.exe", savedDir)
					if err := cmd.Start(); err != nil {
						status.SetText("Could not open the output folder.")
					} else {
						_ = cmd.Process.Release()
					}
				}},
				PushButton{AssignTo: &cancelButton, Text: "Cancel", Visible: false, Enabled: false, OnClicked: func() {
					if cancel != nil {
						cancel()
						cancelButton.SetEnabled(false)
						status.SetText("Canceling conversion…")
						updateProgress(progressAmount, "Canceling")
					}
				}},
				PushButton{AssignTo: &convert, Text: "Create EBOOT.PBP", MinSize: Size{Width: 170, Height: 38}, Font: Font{Family: "Segoe UI", PointSize: 10, Bold: true}, OnClicked: run},
			}},
		},
	}).Create()
	if err != nil {
		return err
	}
	useDarkTitleBar(window)
	for _, button := range []*walk.PushButton{navConvert, navArtwork, navSettings} {
		themeButton(button, sidebar.Color, button == navConvert)
	}
	for _, button := range []*walk.PushButton{logButton, openOutput, cancelButton, convert} {
		themeButton(button, panel.Color, button == convert)
	}
	themeButton(browseOutput, background.Color, false)
	themeButton(removeDisc, field.Color, false)
	dropZone.DropFiles().Attach(dropImages)
	discRow.DropFiles().Attach(dropImages)
	idleDrop.DropFiles().Attach(dropImages)
	dropTitle.DropFiles().Attach(dropImages)
	dropHint.DropFiles().Attach(dropImages)
	dropIcon.DropFiles().Attach(dropImages)
	discName.DropFiles().Attach(dropImages)
	discDetails.DropFiles().Attach(dropImages)
	artPage.start()
	window.Closing().Attach(func(canceled *bool, _ walk.CloseReason) {
		if cancel != nil {
			*canceled = true
			cancel()
			cancelButton.SetEnabled(false)
			status.SetText("Canceling conversion…")
			updateProgress(progressAmount, "Canceling")
			return
		}
		closing = true
	})
	fitWorkArea(window)
	window.Run()
	return nil
}

func useDarkTitleBar(form walk.Form) {
	proc := windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	if proc.Find() != nil {
		return
	}
	const useImmersiveDarkMode = 20
	enabled := uint32(1)
	_, _, _ = proc.Call(uintptr(form.Handle()), useImmersiveDarkMode, uintptr(unsafe.Pointer(&enabled)), unsafe.Sizeof(enabled))
}

func fitWorkArea(window *walk.MainWindow) {
	monitor := win.MonitorFromWindow(window.Handle(), win.MONITOR_DEFAULTTONEAREST)
	info := win.MONITORINFO{CbSize: uint32(unsafe.Sizeof(win.MONITORINFO{}))}
	if !win.GetMonitorInfo(monitor, &info) {
		return
	}
	area := info.RcWork
	width, height := int(area.Right-area.Left), int(area.Bottom-area.Top)
	size := window.BoundsPixels()
	size.Width, size.Height = min(size.Width, width), min(size.Height, height)
	size.X, size.Y = int(area.Left)+(width-size.Width)/2, int(area.Top)+(height-size.Height)/2
	_ = window.SetBoundsPixels(size)
}
