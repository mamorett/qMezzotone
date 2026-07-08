package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mamorett/qMezzotone/internal/app"
	"github.com/mamorett/qMezzotone/internal/services"

	tea "charm.land/bubbletea/v2"
)

func main() {
	debug := flag.Bool("debug", false, "enable debug logging")
	fontTTF := flag.String("font-ttf", "", "path to a .ttf font used for image/gif export rendering")
	flag.Parse()
	if *debug {
		err := services.InitLogger("logs.log")
		if err != nil {
			return
		}
	}

	imagePath := ""
	if args := flag.Args(); len(args) > 0 {
		imagePath = args[0]
	}

	p := tea.NewProgram(app.NewQMezzotoneModelWithConfig(app.QMezzotoneModelConfig{
		ExportFontTTFPath: *fontTTF,
		ImagePath:         imagePath,
	}))
	if _, err := p.Run(); err != nil {
		_ = services.Logger().Error("Unexpected Error. Unable to recover")
		fmt.Printf("An unexpected error has occurred.\n")
		os.Exit(1)
	}
}
