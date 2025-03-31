package screen

import (
	"log"
	"time"
)

func (s *Screen) redrawOp(op string, args *opArgs) {
	// defer func() {
	// 	if r := recover(); r != nil {
	// 		log.Println("PANIC:", r)
	// 	}
	// }()

	switch op {
	case "resize":
		s.setSize(args.Int(), args.Int())
		s.writeSize()

	case "clear":
		s.clearScreen()
		s.Cursor.X = 0
		s.Cursor.Y = 0

		s.writeClear()

	case "flush":
		s.flushScreen(false)

	case "eol_clear":
		s.clearLine(s.Cursor.X, s.Cursor.Y)

	case "cursor_goto":
		y := args.Int()
		x := args.Int()
		s.setCursor(x, y)

	case "update_fg":
		s.DefaultAttrs.Fg = Color(args.Int())

	case "update_bg":
		s.DefaultAttrs.Bg = Color(args.Int())

	case "update_sp":
		s.DefaultAttrs.Sp = Color(args.Int())

	case "highlight_set":
		m := args.Map()
		attrs := *s.DefaultAttrs

		if c, ok := m.Int64("foreground"); ok {
			attrs.Fg = Color(c)
		}

		if c, ok := m.Int64("background"); ok {
			attrs.Bg = Color(c)
		}

		if c, ok := m.Int64("special"); ok {
			attrs.Sp = Color(c)
		}

		if b, ok := m.Bool("reverse"); ok && b {
			attrs.Attrs |= AttrReverse
		}

		if b, ok := m.Bool("italic"); ok && b {
			attrs.Attrs |= AttrItalic
		}

		if b, ok := m.Bool("bold"); ok && b {
			attrs.Attrs |= AttrBold
		}

		if b, ok := m.Bool("underline"); ok && b {
			attrs.Attrs |= AttrUnderline
		}

		if b, ok := m.Bool("undercurl"); ok && b {
			attrs.Attrs |= AttrUndercurl
		}

		// Try to reuse a pointer to an existing color that matches the one that was
		// just set.
		for existing := range s.attrCounter {
			e := *existing
			e.id = 0
			if attrs == e {
				s.CurAttrs = existing
				return
			}
		}

		s.attrID++
		attrs.id = s.attrID
		s.CurAttrs = &attrs

	case "put":
		i := s.Cursor.Y*s.Size.X + s.Cursor.X
		for _, c := range args.String() {
			s.setChar(i, c)
			i++
		}

	case "set_scroll_region":
		s.scroll.tl.Y = args.Int()
		s.scroll.br.Y = args.Int()
		s.scroll.tl.X = args.Int()
		s.scroll.br.X = args.Int()

	case "scroll":
		amount := args.Int()
		sr := s.scroll
		blank := make([]Cell, (sr.br.X-sr.tl.X)+1)
		for i := range blank {
			blank[i].Char = ' '
			blank[i].Sent = false
			blank[i].CellAttrs = s.DefaultAttrs
		}

		ys := amount
		h := (sr.br.Y - sr.tl.Y) + 1
		if amount < 0 {
			// Down
			ys = -amount
		}

		var sy, dy int

		// Copying must go from top to bottom regardless of the scroll direction.
		for y := ys; y < h; y++ {
			if amount < 0 {
				dy = sr.br.Y + ys - y
				sy = dy + amount
			} else {
				sy = sr.tl.Y + y
				dy = sy - amount
			}

			sy *= s.Size.X
			dy *= s.Size.X

			src := s.Buffer[sy+sr.tl.X : sy+sr.br.X+1]
			dst := s.Buffer[dy+sr.tl.X : dy+sr.br.X+1]

			copy(dst, src)
			copy(src, blank) // Always blank the source line.
		}

		s.writeScroll(amount)

	case "set_title":
		s.Title = args.String()
		s.writeTitle(s.Title)
		log.Println("set_title")

	case "set_icon":
		s.writeIcon(args.String())
		log.Println("set_icon")

	case "mouse_on":
		s.Mouse = true
		log.Println("Mouse Enabled")

	case "mouse_off":
		s.Mouse = false
		log.Println("Mouse Disabled")

	case "busy_start":
		fallthrough
	case "busy_on":
		s.Busy = true
		log.Println("Busy")

	case "busy_stop":
		fallthrough
	case "busy_off":
		s.Busy = false
		log.Println("Not Busy")

	case "suspend":
		log.Println("suspend")

	case "bell":
		log.Println("Bell")
		s.writeBell(false)

	case "visual_bell":
		log.Println("Visual bell")
		s.writeBell(true)

	case "mode_change":
		mode := args.String()
		log.Println("Mode change:", mode)

		switch mode {
		case "normal":
			s.Mode = ModeNormal
		case "insert":
			s.Mode = ModeInsert
		case "replace":
			s.Mode = ModeReplace
		}

	case "popupmenu_show":
		// TODO

	case "popupmenu_select":

	case "popupmenu_hide":

	case "win_viewport":

	default:
		log.Printf("Unknown redraw op: %s, %#v", op, args.args)
	}
}

func (s *Screen) redrawFinalize(curFlush int) {
	<-time.After(time.Millisecond * 5)
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.flushCount != curFlush {
		return
	}

	if err := s.flush(true); err != nil {
		log.Println("Couldn't flush data:", err)
	}
}

// RedrawHandler deals with the msgpack input from Neovim
func (s *Screen) RedrawHandler(updates ...[]interface{}) {
	s.mu.Lock()
	s.flushCount++

oploop:
	for _, args := range updates {
		var op string
		switch n := args[0].(type) {
		case string:
			op = n
		default:
			log.Println("Unknown Op:", n)
			break oploop
		}

		for _, u := range args[1:] {
			switch a := u.(type) {
			case []interface{}:
				s.redrawOp(op, &opArgs{args: a})

			default:
				log.Printf("Unknown arguments for op '%s': %#v\n", op, a)
			}

		}
	}

	s.flushScreen(false)
	if err := s.flush(false); err != nil {
		log.Println("Couldn't flush data:", err)
	}
	s.mu.Unlock()
	go s.redrawFinalize(s.flushCount)
}
