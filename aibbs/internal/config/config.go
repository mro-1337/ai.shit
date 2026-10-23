// Package config loads the JSON configuration that drives an AI BBS user.
//
// JSON was chosen deliberately: it is in the Go standard library, so the
// binary has no third-party dependencies at all.
package config

// Config is the whole configuration file.
type Config struct {
	BBS          BBSConfig          `json:"bbs"`
	Account      AccountConfig      `json:"account"`
	AI           AIConfig           `json:"ai"`
	Behavior     BehaviorConfig     `json:"behavior"`
	Registration RegistrationConfig `json:"registration"`
	Downloads    DownloadsConfig    `json:"downloads"`
	Doors        DoorsConfig        `json:"doors"`
}

// BBSConfig describes the target system.
type BBSConfig struct {
	Name             string `json:"name"`
	Host             string `json:"host"`
	Port             int    `json:"port"`
	Cols             int    `json:"cols"`
	Rows             int    `json:"rows"`
	Terminal         string `json:"terminal"`
	DialTimeoutSec   int    `json:"dial_timeout_sec"`
	IdleMillis       int    `json:"idle_millis"`
	MaxSettleSeconds int    `json:"max_settle_seconds"`
}

// AccountConfig is the login we want to use (and create if needed).
type AccountConfig struct {
	Handle        string `json:"handle"`
	Password      string `json:"password"`
	PasswordEnv   string `json:"password_env"`
	RealName      string `json:"real_name"`
	Email         string `json:"email"`
	Location      string `json:"location"`
	AllowRegister bool   `json:"allow_register"`
}

// AIConfig selects the language model. Any OpenAI compatible endpoint works
// (OpenAI, OpenRouter, Groq, LM Studio, Ollama's /v1 API, ...).
type AIConfig struct {
	Provider       string            `json:"provider"`
	BaseURL        string            `json:"base_url"`
	APIKey         string            `json:"api_key"`
	APIKeyEnv      string            `json:"api_key_env"`
	Model          string            `json:"model"`
	Temperature    float64           `json:"temperature"`
	MaxTokens      int               `json:"max_tokens"`
	TimeoutSec     int               `json:"timeout_sec"`
	JSONMode       bool              `json:"json_mode"`
	Headers        map[string]string `json:"headers,omitempty"`
	ExtraPrompt    string            `json:"extra_prompt"`
	TrimScreenRows int               `json:"trim_screen_rows"`
}

// BehaviorConfig controls etiquette, limits and the persona.
type BehaviorConfig struct {
	Persona                string   `json:"persona"`
	Style                  string   `json:"style"`
	Goals                  []string `json:"goals"`
	Interests              []string `json:"interests"`
	IgnoreTopics           []string `json:"ignore_topics"`
	BlockedWords           []string `json:"blocked_words"`
	MaxSteps               int      `json:"max_steps"`
	MinActionDelayMS       int      `json:"min_action_delay_ms"`
	MaxPostsPerSession     int      `json:"max_posts_per_session"`
	MaxDownloadsPerSession int      `json:"max_downloads_per_session"`
	MaxDownloadMB          int      `json:"max_download_mb"`
	MaxReadMessages        int      `json:"max_read_messages"`
	MinPostChars           int      `json:"min_post_chars"`
	MaxPostChars           int      `json:"max_post_chars"`
	DryRun                 bool     `json:"dry_run"`
	ReadOnly               bool     `json:"read_only"`
	AllowDoors             bool     `json:"allow_doors"`
	AutoPager              bool     `json:"auto_pager"`
	StateDir               string   `json:"state_dir"`
	Transcript             bool     `json:"transcript"`
}

// RegistrationConfig tells the bot how to answer sign-up questions.
type RegistrationConfig struct {
	NewUserCommand string       `json:"new_user_command"`
	TerminalAnswer string       `json:"terminal_answer"`
	PasswordLength int          `json:"password_length"`
	Answers        []AnswerRule `json:"answers"`
}

// AnswerRule maps a prompt (regular expression) to an answer.
type AnswerRule struct {
	Match string `json:"match"`
	Value string `json:"value"`
}

// DownloadsConfig controls file transfers.
type DownloadsConfig struct {
	Dir          string            `json:"dir"`
	Protocols    []string          `json:"protocols"`
	ProtocolKeys map[string]string `json:"protocol_keys"`
	IdleSeconds  int               `json:"idle_seconds"`
	MaxFileMB    int               `json:"max_file_mb"`
}

// DoorsConfig controls door game play.
type DoorsConfig struct {
	Enabled    bool         `json:"enabled"`
	MaxActions int          `json:"max_actions"`
	Overrides  []DoorConfig `json:"overrides"`
}

// DoorConfig customises a known door game or adds a new one.
type DoorConfig struct {
	Name     string `json:"name"`
	Match    string `json:"match"`
	Launch   string `json:"launch"`
	Controls string `json:"controls"`
	Strategy string `json:"strategy"`
}
