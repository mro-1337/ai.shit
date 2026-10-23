// Package doors describes the door games commonly found on BBSes so the
// agent knows how to launch them and what the controls mean.
package doors

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mro-1337/ai.shit/aibbs/internal/config"
)

// Profile is a door game the agent can recognise.
type Profile struct {
	Name       string `json:"name"`
	Match      string `json:"match"`
	Launch     string `json:"launch"`
	Controls   string `json:"controls"`
	Strategy   string `json:"strategy"`
	MaxActions int    `json:"max_actions"`
}

// DefaultProfiles covers the games that show up on most Synchronet and
// Mystic boards.
func DefaultProfiles() []Profile {
	return []Profile{
		{
			Name:     "Legend of the Red Dragon",
			Match:    `(?i)legend of the red dragon|\bLORD\b`,
			Launch:   "L",
			Controls: "L=look, F=fight, I=inn, B=bank, S=shop, Q=quit. Menus are single keys.",
			Strategy: "Create a character if asked. Fight weaker monsters, rest at the inn when hurt, " +
				"bank gold before quitting. Keep it to a few turns.",
			MaxActions: 30,
		},
		{
			Name:     "TradeWars 2002",
			Match:    `(?i)trade ?wars`,
			Launch:   "T",
			Controls: "C=computer, P=port, S=scan, M=move, R=report, Q=quit.",
			Strategy: "Start with a few trading runs between ports, note profitable routes, and do not " +
				"pick fights with players who outclass you.",
			MaxActions: 40,
		},
		{
			Name:       "Usurper",
			Match:      `(?i)usurper`,
			Launch:     "U",
			Controls:   "Single-key menus; L=look, F=fight, S=shop, T=tavern.",
			Strategy:   "Train a little, fight safe opponents, and avoid spending all your gold at once.",
			MaxActions: 30,
		},
		{
			Name:       "Barren Realms Elite",
			Match:      `(?i)barren realms`,
			Launch:     "B",
			Controls:   "Numbered menus; follow the prompts for building and attacking.",
			Strategy:   "Invest in economy early, keep turns in reserve, and do not attack larger realms.",
			MaxActions: 30,
		},
		{
			Name:       "Global War",
			Match:      `(?i)global war`,
			Launch:     "G",
			Controls:   "Numbered menus; buy troops, then attack or defend.",
			Strategy:   "Balance economy and defence; a few cautious attacks beat one reckless one.",
			MaxActions: 30,
		},
		{
			Name:       "Operation Overkill",
			Match:      `(?i)operation overkill`,
			Launch:     "O",
			Controls:   "Numbered menus; explore, fight, buy gear.",
			Strategy:   "Explore carefully and upgrade equipment before deeper runs.",
			MaxActions: 30,
		},
		{
			Name:       "PimpWars",
			Match:      `(?i)pimp ?wars`,
			Launch:     "P",
			Controls:   "Single-key menus; buy, sell, and manage your crew.",
			Strategy:   "Keep a cash buffer and avoid risky fights early.",
			MaxActions: 30,
		},
		{
			Name:       "Phantasia",
			Match:      `(?i)phantasia`,
			Launch:     "P",
			Controls:   "Single-key menus; explore, fight monsters, visit the inn.",
			Strategy:   "Fight monsters near your level and retreat when low on hit points.",
			MaxActions: 30,
		},
	}
}

// WithOverrides merges configured door definitions into the defaults.
func WithOverrides(overrides []config.DoorConfig) []Profile {
	profiles := DefaultProfiles()
	for _, o := range overrides {
		if o.Name == "" {
			continue
		}
		found := false
		for i := range profiles {
			if strings.EqualFold(profiles[i].Name, o.Name) {
				applyOverride(&profiles[i], o)
				found = true
				break
			}
		}
		if !found {
			profiles = append(profiles, Profile{
				Name: o.Name, Match: o.Match, Launch: o.Launch,
				Controls: o.Controls, Strategy: o.Strategy, MaxActions: 30,
			})
		}
	}
	return profiles
}

func applyOverride(p *Profile, o config.DoorConfig) {
	if o.Match != "" {
		p.Match = o.Match
	}
	if o.Launch != "" {
		p.Launch = o.Launch
	}
	if o.Controls != "" {
		p.Controls = o.Controls
	}
	if o.Strategy != "" {
		p.Strategy = o.Strategy
	}
}

// Detect returns the first profile whose pattern matches the screen.
func Detect(profiles []Profile, screen string) *Profile {
	for i := range profiles {
		if profiles[i].Match == "" {
			continue
		}
		re, err := regexp.Compile(profiles[i].Match)
		if err != nil {
			continue
		}
		if re.MatchString(screen) {
			return &profiles[i]
		}
	}
	return nil
}

// LaunchHint describes how to start the door from the current menu.
func (p *Profile) LaunchHint() string {
	if p.Launch == "" {
		return ""
	}
	return fmt.Sprintf("To start %s from a menu, send %q or look for its name in the menu.", p.Name, p.Launch)
}

// Brief renders the profile for the model prompt.
func (p *Profile) Brief() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Door game detected: %s\n", p.Name)
	if p.Controls != "" {
		fmt.Fprintf(&b, "Controls: %s\n", p.Controls)
	}
	if p.Strategy != "" {
		fmt.Fprintf(&b, "Strategy: %s\n", p.Strategy)
	}
	return strings.TrimRight(b.String(), "\n")
}
