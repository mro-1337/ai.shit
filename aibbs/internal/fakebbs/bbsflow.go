package fakebbs

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/mro-1337/ai.shit/aibbs/internal/transfer"
)

type connState struct {
	srv  *Server
	t    *tConn
	user *User
}

func (c *connState) run() {
	c.banner()
	if !c.login() {
		c.t.writeString("\r\nGoodbye!\r\n")
		return
	}
	c.mainMenu()
	c.t.writeString("\r\nLogging off... Goodbye!\r\n")
}

func (c *connState) banner() {
	c.t.writeString("\x1b[2J\x1b[H")
	c.t.writeString(bannerLine("+==========================================+"))
	c.t.writeString(bannerLine("|        A I   B B S   T E S T             |"))
	c.t.writeString(bannerLine("+==========================================+"))
	c.t.writeString("\x1b[0;37mA safe playground for an AI BBS user.\x1b[0m\r\n\r\n")
}

func (c *connState) login() bool {
	for attempt := 0; attempt < 4; attempt++ {
		c.t.writeString("\r\nEnter your name (or NEW to register): ")
		name, err := c.t.readLine(2 * time.Minute)
		if err != nil || name == "" {
			return false
		}
		if strings.EqualFold(name, "new") {
			if c.register("") {
				return true
			}
			continue
		}
		c.srv.mu.Lock()
		user := c.srv.users[strings.ToLower(name)]
		c.srv.mu.Unlock()
		if user != nil {
			if c.askPassword(user) {
				c.user = user
				return true
			}
			return false
		}
		c.t.writef("No account by the name %q. Create it? (Y/N): ", name)
		answer, err := c.t.readLine(2 * time.Minute)
		if err != nil {
			return false
		}
		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "Y") {
			if c.register(name) {
				return true
			}
		} else {
			return false
		}
	}
	return false
}

func (c *connState) askPassword(u *User) bool {
	for i := 0; i < 3; i++ {
		c.t.writeString("Password: ")
		pw, err := c.t.readLine(2 * time.Minute)
		if err != nil {
			return false
		}
		if pw == u.Password {
			return true
		}
		c.t.writeString("Incorrect password.\r\n")
	}
	return false
}

func (c *connState) register(handle string) bool {
	if handle == "" {
		for {
			c.t.writeString("\r\nChoose a handle: ")
			name, err := c.t.readLine(2 * time.Minute)
			if err != nil || name == "" {
				return false
			}
			if len(name) > 24 {
				c.t.writeString("That handle is too long (24 characters max).\r\n")
				continue
			}
			c.srv.mu.Lock()
			_, exists := c.srv.users[strings.ToLower(name)]
			c.srv.mu.Unlock()
			if exists {
				c.t.writeString("That handle is already taken.\r\n")
				continue
			}
			handle = name
			break
		}
	}

	var password string
	for attempt := 0; attempt < 3; attempt++ {
		c.t.writeString("Choose a password: ")
		first, err := c.t.readLine(2 * time.Minute)
		if err != nil {
			return false
		}
		if len(first) < 4 {
			c.t.writeString("Password must be at least 4 characters.\r\n")
			continue
		}
		c.t.writeString("Verify password: ")
		second, err := c.t.readLine(2 * time.Minute)
		if err != nil {
			return false
		}
		if first != second {
			c.t.writeString("Passwords do not match. Try again.\r\n")
			continue
		}
		password = first
		break
	}
	if password == "" {
		return false
	}

	c.t.writeString("Real name: ")
	realName, _ := c.t.readLine(2 * time.Minute)
	c.t.writeString("Email: ")
	email, _ := c.t.readLine(2 * time.Minute)
	c.t.writeString("Location: ")
	location, _ := c.t.readLine(2 * time.Minute)
	c.t.writeString("Terminal (A=ANSI, R=ASCII): ")
	if _, err := c.t.readLine(2 * time.Minute); err != nil {
		return false
	}
	if realName == "" {
		realName = handle
	}
	u := &User{Handle: handle, Password: password, RealName: realName, Email: email, Location: location}
	c.srv.mu.Lock()
	c.srv.users[strings.ToLower(handle)] = u
	c.srv.stats.Registrations++
	c.srv.mu.Unlock()
	c.user = u
	c.t.writef("\r\nAccount created. Welcome, %s!\r\n", handle)
	return true
}

func (c *connState) mainMenu() {
	for {
		c.t.writeString("\x1b[2J\x1b[H")
		c.t.writeString(bannerLine("+==========================================+"))
		c.t.writeString(bannerLine("|           AI BBS TEST SYSTEM             |"))
		c.t.writeString(bannerLine("+==========================================+"))
		c.t.writef("\x1b[0;37m  Main Menu              User: %-12s\x1b[0m\r\n\r\n", c.user.Handle)
		c.t.writeString("  M) Message Areas\r\n")
		c.t.writeString("  F) File Areas\r\n")
		c.t.writeString("  G) Door Games\r\n")
		c.t.writeString("  U) User List\r\n")
		c.t.writeString("  Q) Log off\r\n\r\n")
		c.t.writeString("  Choice: ")

		key, err := c.t.readKey(5 * time.Minute)
		if err != nil {
			return
		}
		switch strings.ToUpper(string(key)) {
		case "M":
			c.messageMenu()
		case "F":
			c.fileMenu()
		case "G":
			c.doorMenu()
		case "U":
			c.userList()
		case "Q":
			c.srv.bump(func(s *Stats) { s.Logoffs++ })
			return
		case "\r", "\n":
			// stray newline from a client that sends lines instead of keys
		default:
			c.t.writeString("\r\nUnknown command.\r\n")
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (c *connState) userList() {
	c.t.writeString("\x1b[2J\x1b[H")
	c.t.writeString(bannerLine("Registered users"))
	c.srv.mu.Lock()
	lines := make([]string, 0, len(c.srv.users))
	for _, u := range c.srv.users {
		lines = append(lines, fmt.Sprintf("  %-20s %s", u.Handle, u.Location))
	}
	c.srv.mu.Unlock()
	for i := 0; i < len(lines); i++ {
		for j := i + 1; j < len(lines); j++ {
			if lines[j] < lines[i] {
				lines[i], lines[j] = lines[j], lines[i]
			}
		}
	}
	lines = append(lines, "", "Hit any key to continue.")
	c.paged(lines)
	c.t.readKey(2 * time.Minute)
}

// ---------------------------------------------------------------- messages

var areas = []string{"General", "Tech", "Games"}

func (c *connState) messageMenu() {
	for {
		c.t.writeString("\x1b[2J\x1b[H")
		c.t.writeString(bannerLine("Message Areas"))
		for i, area := range areas {
			c.srv.mu.Lock()
			count := 0
			for _, m := range c.srv.msgs {
				if m.Area == area {
					count++
				}
			}
			c.srv.mu.Unlock()
			c.t.writef("  %d) %-12s (%d messages)\r\n", i+1, area, count)
		}
		c.t.writeString("  Q) Back to main menu\r\n\r\nArea: ")
		line, err := c.t.readLine(5 * time.Minute)
		if err != nil {
			return
		}
		switch strings.ToUpper(line) {
		case "Q", "":
			return
		case "1", "2", "3":
			n, _ := strconv.Atoi(line)
			c.areaView(areas[n-1])
		default:
			c.t.writeString("No such area.\r\n")
		}
	}
}

func (c *connState) areaView(area string) {
	for {
		c.srv.mu.Lock()
		msgs := make([]Message, 0)
		for _, m := range c.srv.msgs {
			if m.Area == area {
				msgs = append(msgs, *m)
			}
		}
		c.srv.mu.Unlock()

		c.t.writeString("\x1b[2J\x1b[H")
		c.t.writeString(bannerLine(fmt.Sprintf("%s - %d messages", area, len(msgs))))
		lines := make([]string, 0, len(msgs)+2)
		for _, m := range msgs {
			lines = append(lines, fmt.Sprintf("  %2d) %-34s from %-12s", m.ID, m.Subject, m.From))
		}
		lines = append(lines, "", "Read #, [P]ost, or [Q]uit.")
		c.paged(lines)
		c.t.writeString("Read # / P / Q: ")
		line, err := c.t.readLine(5 * time.Minute)
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		switch strings.ToUpper(line) {
		case "Q", "":
			return
		case "P":
			c.compose(area, "")
		default:
			id, err := strconv.Atoi(line)
			if err != nil {
				c.t.writeString("Enter a message number, P, or Q.\r\n")
				continue
			}
			c.showMessage(area, id)
		}
	}
}

func (c *connState) showMessage(area string, id int) {
	var msg *Message
	c.srv.mu.Lock()
	for _, m := range c.srv.msgs {
		if m.ID == id && m.Area == area {
			cp := *m
			msg = &cp
			break
		}
	}
	c.srv.mu.Unlock()
	if msg == nil {
		c.t.writeString("No such message.\r\n")
		return
	}
	c.srv.bump(func(s *Stats) { s.Reads++ })

	c.t.writeString("\x1b[2J\x1b[H")
	c.t.writeString(bannerLine(fmt.Sprintf("Message #%d in %s", msg.ID, msg.Area)))
	c.t.writef("\x1b[0;37mFrom: %s\r\nTo:   %s\r\nSubj: %s\r\nDate: %s\r\n", msg.From, msg.To, msg.Subject, msg.At.Format(time.RFC822))
	c.t.writeString(strings.Repeat("-", 70) + "\x1b[0m\r\n")
	lines := strings.Split(msg.Body, "\r\n")
	lines = append(lines, "")
	c.paged(lines)

	c.t.writeString("[R]eply, [N]ext, or [Q]uit: ")
	line, err := c.t.readLine(5 * time.Minute)
	if err != nil {
		return
	}
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(line)), "R") {
		c.compose(area, "Re: "+msg.Subject)
	}
}

func (c *connState) compose(area, subject string) {
	if subject == "" {
		c.t.writeString("\r\nSubject: ")
		s, err := c.t.readLine(2 * time.Minute)
		if err != nil {
			return
		}
		subject = strings.TrimSpace(s)
		if subject == "" {
			c.t.writeString("Post cancelled.\r\n")
			return
		}
	}
	c.t.writeString("Enter your message. End with a line containing only a period (.).\r\n")
	var body []string
	for i := 0; i < 200; i++ {
		line, err := c.t.readLine(5 * time.Minute)
		if err != nil {
			return
		}
		if line == "." {
			break
		}
		body = append(body, line)
	}
	text := strings.TrimSpace(strings.Join(body, "\r\n"))
	if text == "" {
		c.t.writeString("Nothing to post.\r\n")
		return
	}
	c.t.writeString("Save this message? (Y/N): ")
	answer, err := c.t.readLine(2 * time.Minute)
	if err != nil {
		return
	}
	if !strings.HasPrefix(strings.ToUpper(strings.TrimSpace(answer)), "Y") {
		c.t.writeString("Message discarded.\r\n")
		return
	}
	msg := c.srv.addMessage(area, c.user.Handle, subject, text)
	c.t.writef("Message posted as #%d in %s.\r\n", msg.ID, area)
	c.t.writeString("Hit any key to continue.")
	_, _ = c.t.readKey(2 * time.Minute)
}

// ------------------------------------------------------------------- files

func (c *connState) fileMenu() {
	for {
		files := c.srv.fileList()
		c.t.writeString("\x1b[2J\x1b[H")
		c.t.writeString(bannerLine("File Areas"))
		c.t.writeString("  #  Filename             Size   Description\r\n")
		for i, f := range files {
			c.t.writef("  %d  %-20s %-6d %s\r\n", i+1, f.Name, len(f.Data), f.Desc)
		}
		c.t.writeString("  Q) Back to main menu\r\n\r\n")
		c.t.writeString("Download which file (#) or Q: ")
		line, err := c.t.readLine(5 * time.Minute)
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if strings.EqualFold(line, "q") || line == "" {
			return
		}
		n, err := strconv.Atoi(line)
		if err != nil || n < 1 || n > len(files) {
			c.t.writeString("No such file.\r\n")
			continue
		}
		f := files[n-1]
		c.t.writeString("Protocol (Z=ZMODEM, X=XMODEM, Y=YMODEM, Q=Quit): ")
		key, err := c.t.readKey(2 * time.Minute)
		if err != nil {
			return
		}
		switch strings.ToUpper(string(key)) {
		case "Z", "X", "Y":
			c.transferFile(f, strings.ToUpper(string(key)))
		case "Q":
			return
		default:
			c.t.writeString("Unknown protocol.\r\n")
			continue
		}
		c.t.writeString("\r\nHit any key to continue.")
		_, _ = c.t.readKey(2 * time.Minute)
	}
}

func (c *connState) transferFile(f *FileEntry, protocol string) {
	c.t.writef("\r\nStarting %s transfer of %s (%d bytes).\r\n", protocol, f.Name, len(f.Data))
	stream := doorStream{t: c.t}
	var err error
	switch protocol {
	case "Z":
		err = transfer.SendZModem(stream, []transfer.OutFile{{Name: f.Name, Data: f.Data}}, 60*time.Second, nil)
	case "Y":
		err = transfer.SendXModem(stream, []transfer.OutFile{{Name: f.Name, Data: f.Data}}, true, 60*time.Second, nil)
	default:
		err = transfer.SendXModem(stream, []transfer.OutFile{{Name: f.Name, Data: f.Data}}, false, 60*time.Second, nil)
	}
	if err != nil {
		c.t.writef("\r\nTransfer failed: %v\r\n", err)
		return
	}
	c.srv.bump(func(s *Stats) { s.Transfers++ })
	c.t.writeString("\r\nTransfer complete.\r\n")
}

// doorStream adapts the server connection to transfer.Stream.
type doorStream struct{ t *tConn }

func (d doorStream) ReadByteTimeout(timeout time.Duration) (byte, error) {
	b, err := d.t.readByte(timeout)
	if err != nil && isTimeout(err) {
		return 0, transfer.ErrTimeout
	}
	return b, err
}

func (d doorStream) Write(data []byte) error {
	d.t.writeRaw(data)
	return nil
}

// ------------------------------------------------------------------- doors

func (c *connState) doorMenu() {
	for {
		c.t.writeString("\x1b[2J\x1b[H")
		c.t.writeString(bannerLine("Door Games"))
		c.t.writeString("  1) Guess the Number\r\n")
		c.t.writeString("  2) Dice Duel\r\n")
		c.t.writeString("  Q) Back to main menu\r\n\r\nChoice: ")
		line, err := c.t.readLine(5 * time.Minute)
		if err != nil {
			return
		}
		switch strings.ToUpper(strings.TrimSpace(line)) {
		case "1":
			c.guessGame()
		case "2":
			c.diceGame()
		case "Q", "":
			return
		default:
			c.t.writeString("No such game.\r\n")
		}
	}
}

func (c *connState) guessGame() {
	secret := rand.Intn(100) + 1
	c.t.writeString("\x1b[2J\x1b[H")
	c.t.writeString(bannerLine("Guess the Number"))
	c.t.writeString("I am thinking of a number between 1 and 100. You get 7 guesses.\r\n\r\n")
	guesses := 0
	won := false
	for guesses < 7 {
		c.t.writeString("Your guess: ")
		line, err := c.t.readLine(5 * time.Minute)
		if err != nil {
			return
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > 100 {
			c.t.writeString("Enter a number from 1 to 100.\r\n")
			continue
		}
		guesses++
		switch {
		case n == secret:
			c.t.writef("Correct! You got it in %d guesses.\r\n", guesses)
			won = true
		case n < secret:
			c.t.writeString("Higher.\r\n")
		default:
			c.t.writeString("Lower.\r\n")
		}
		if won {
			break
		}
	}
	if !won {
		c.t.writef("Out of guesses. The number was %d.\r\n", secret)
	}
	c.srv.bump(func(s *Stats) { s.DoorPlays++ })
	c.t.writeString("\r\nHit any key to return to the door menu.")
	_, _ = c.t.readKey(2 * time.Minute)
}

func (c *connState) diceGame() {
	c.t.writeString("\x1b[2J\x1b[H")
	c.t.writeString(bannerLine("Dice Duel"))
	c.t.writeString("I roll two dice. Guess whether the total is High (8-12) or Low (2-6).\r\n")
	c.t.writeString("A total of 7 is a push. Five rounds, then we stop.\r\n\r\n")
	wins, losses := 0, 0
	for round := 1; round <= 5; round++ {
		c.t.writef("Round %d - High, Low, or Q to quit (H/L/Q): ", round)
		key, err := c.t.readKey(2 * time.Minute)
		if err != nil {
			return
		}
		choice := strings.ToUpper(string(key))
		if choice == "Q" {
			break
		}
		if choice != "H" && choice != "L" {
			c.t.writeString("Pick H or L.\r\n")
			round--
			continue
		}
		d1, d2 := rand.Intn(6)+1, rand.Intn(6)+1
		total := d1 + d2
		c.t.writef("You rolled %d and %d for a total of %d. ", d1, d2, total)
		switch {
		case total == 7:
			c.t.writeString("Push!\r\n")
		case (total > 7 && choice == "H") || (total < 7 && choice == "L"):
			c.t.writeString("You win!\r\n")
			wins++
		default:
			c.t.writeString("You lose.\r\n")
			losses++
		}
	}
	c.t.writef("\r\nFinal score: %d wins, %d losses.\r\n", wins, losses)
	c.srv.bump(func(s *Stats) { s.DoorPlays++ })
	c.t.writeString("\r\nHit any key to return to the door menu.")
	_, _ = c.t.readKey(2 * time.Minute)
}

// ----------------------------------------------------------------- helpers

// paged prints lines, pausing with a -- More -- prompt every 20 lines.
func (c *connState) paged(lines []string) {
	count := 0
	for _, line := range lines {
		c.t.writef("%s\r\n", line)
		count++
		if count >= 20 {
			c.t.writeString("\x1b[0;37m-- More --\x1b[0m")
			if _, err := c.t.readKey(2 * time.Minute); err != nil {
				return
			}
			count = 0
		}
	}
}
