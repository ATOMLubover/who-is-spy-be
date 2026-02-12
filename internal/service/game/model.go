package game

// 玩家身份
const (
	ROLE_UNSET    = "Unset"
	ROLE_ADMIN    = "Admin"
	ROLE_NORMAL   = "Normal"
	ROLE_BLANK    = "Blank"
	ROLE_SPY      = "Spy"
	ROLE_OBSERVER = "Observer"
	ROLE_OB_NORMAL = "ObNormal" // eliminated normal, kept for server-internal tracking
	ROLE_OB_SPY    = "ObSpy"    // eliminated spy, kept for server-internal tracking
	ROLE_OB_BLANK  = "ObBlank"  // eliminated blank, kept for server-internal tracking
)

type Player struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
	Word string `json:"word,omitempty"`

	// ReqCh  chan RequestWrapper
	RespCh chan ResponseWrapper `json:"-"`
}
