package structs

type User struct {
	Username  string
	Password  string
	Bio       string
	Token     string
	ID        string
	Email     string
	EmailHash string
}

type OTP struct {
	UserID            string
	PasswordGenerated string
	OneTimePassword   string
	Expires           int64
}

type Mod struct {
	ID            string
	Author        string
	Version       int
	Video         string
	Game          string
	Platform      string
	Downloads     int
	Published     bool
	Name          string
	Description   string
	RepositoryUrl string
	Dependencies  []string
	CachedLikes   int
}

type Comment struct {
	ID      string
	Page    string
	Author  string
	Content string
}

type Ticket struct {
	ID            string
	Action        string
	Title         string
	TargetID      string
	Meta          string
	Author        string
	ResultMessage string
	Result        int
}

type Message struct {
	ID      string
	Content string
	From    string
	To      string
}

type MessageElement struct {
	Type  string
	Value string
}

type EmailOptions struct {
	ID       string
	Messages int
}

type Config struct {
	Options map[string]string
}

type RateLimit struct {
	IP         string
	ExpireDate string
}

// Request Structures

type RequestEmailOptions struct {
	Token    string
	Messages string
}

type RequestMessage struct {
	ID    string
	Token string
}

type RequestReport struct {
	Token        string
	ReportReason string
	TargetID     string
}

type RequestLike struct {
	Token  string
	PageID string
}

type RequestComment struct {
	Content string
	ID      string
	PageID  string
	Token   string
}

type RequestRegisterAccount struct {
	Username string `json:"username"`
	ID       string `json:"id"`
	Password string `json:"password"`
	Token    string `json:"token"`
	Email    string `json:"email"`
	Bio      string
}

type RequestToken struct {
	Token string `json:"token"`
}

type RequestModQuery struct {
	AuthorID    string
	Token       string
	SearchQuery string
	Game        string
	Platform    string
	PageIndex   int
	QuerySize   int
	Order       int
}

type RequestModUpload struct {
	GitRepositoryUrl string
	Name             string
	Token            string
	ID               string
	Publish          bool
}

type RequestMod struct {
	ID string
}

type RequestPageComments struct {
	PageID string
}

type RequestGit struct {
	GitUrl string
}

type ResponseLiked struct {
	Liked bool
}

// Misc

type ModMetadata struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Game         string   `json:"game"`
	Platform     string   `json:"platform"`
	IconPath     string   `json:"icon_path"`
	Video        string   `json:"youtubevideo"`
	Downloads    int      `json:"downloads"`
	Dependencies []string `json:"dependencies"`
}

//

type ResponseCommit struct {
	CommitContent string
	Author        string
	Hash          string
	Timestamp     string
}

type ResponseModQuery struct {
	ModObjs      []Mod
	RawQuerySize int
}
