package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jesseduffield/gocui"
)

func isUnknownView(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown view")
}

func main() {
	f, _ := os.Create("/tmp/gocui-log.txt")
	defer f.Close()

	g, err := gocui.NewGui(gocui.NewGuiOpts{OutputMode: gocui.OutputTrue})
	if err != nil {
		fmt.Fprintln(f, "NewGui:", err)
		return
	}
	defer g.Close()

	g.SetManagerFunc(func(g *gocui.Gui) error {
		maxX, maxY := g.Size()
		v, err := g.SetView("test", 0, 0, maxX-1, maxY-1, 0)
		if err != nil && !isUnknownView(err) {
			return err
		}
		if isUnknownView(err) {
			fmt.Fprintln(v, "  It works in Docker!")
			fmt.Fprintln(v, "  gocui + tmux + capture-pane")
			fmt.Fprintln(v, "")
			fmt.Fprintln(v, "  Press q to quit")
		}
		return nil
	})

	g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit)
	g.SetKeybinding("", 'q', gocui.ModNone, quit)

	mainErr := g.MainLoop()
	if mainErr != nil && !isUnknownView(mainErr) && !strings.Contains(mainErr.Error(), "quit") {
		fmt.Fprintln(f, "MainLoop:", mainErr)
	}
	fmt.Fprintln(f, "clean exit")
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}
