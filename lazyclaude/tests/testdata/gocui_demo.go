package main

import (
	"fmt"
	"log"

	"github.com/jesseduffield/gocui"
)

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	splitX := maxX / 3

	if v, err := g.SetView("sessions", 0, 0, splitX-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " [Sessions]  Server "
		fmt.Fprintln(v, "  > my-app          *")
		fmt.Fprintln(v, "    my-lib          -")
		fmt.Fprintln(v, "    srv1:work       *")
		fmt.Fprintln(v, "    old-project     x")
	}

	if v, err := g.SetView("main", splitX, 0, maxX-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Main "
		fmt.Fprintln(v, "  $ claude")
		fmt.Fprintln(v, "  I'll help you with that task...")
		fmt.Fprintln(v, "  > Edit src/main.go")
	}

	if v, err := g.SetView("options", 0, maxY-2, maxX-1, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		fmt.Fprint(v, " n: new  d: del  enter: attach  r: resume  q: quit")
	}

	return nil
}

func main() {
	g, err := gocui.NewGui(gocui.NewGuiOpts{OutputMode: gocui.OutputTrue})
	if err != nil {
		log.Fatal("NewGui: ", err)
	}
	defer g.Close()

	g.SetManagerFunc(layout)

	if err := g.SetKeybinding("", 'q', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	}); err != nil {
		log.Fatal("keybind: ", err)
	}

	if err := g.MainLoop(); err != nil {
		if err == gocui.ErrQuit {
			return
		}
		// go-errors wraps, so check string
		if err.Error() == "unknown view" {
			return
		}
		log.Fatal("MainLoop: ", err)
	}
}
