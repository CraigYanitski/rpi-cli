package spinner

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/charmbracelet/lipgloss"
)

type Spinner struct {
	spinner    *spinner.Spinner
	passStyle  lipgloss.Style
	failStyle  lipgloss.Style
}

func NewSpinner(cs int, d time.Duration, passStyle, failStyle lipgloss.Style, options ...spinner.Option) *Spinner {
	s := spinner.New(spinner.CharSets[cs], d, options...)
	s.Reverse()
	s.Color("magenta", "bold")
	spin := &Spinner{
		spinner: s,
		passStyle: passStyle,
		failStyle: failStyle,
	}
	return spin
}

func (s *Spinner) Start() {
	s.spinner.Start()
}

func (s *Spinner) Stop() {
	s.spinner.Stop()
}

func (s *Spinner) Color(colors ...string) error {
	return s.spinner.Color(colors...)
}

func (s *Spinner) Set(msg string) {
	s.spinner.Suffix = fmt.Sprintf(" %s", msg)
	s.Start()
}

func (s *Spinner) Pass() {
	s.Stop()
	passMsg := s.passStyle.Render("✓" + s.spinner.Suffix)
	fmt.Println(passMsg)
}

func (s *Spinner) Fail() {
	s.Stop()
	failMsg := s.failStyle.Render("✗" + s.spinner.Suffix)
	fmt.Println(failMsg)
}

func (s *Spinner) Clear() {
	s.Stop()
	fmt.Print("\033[K")  //clear current line
}
