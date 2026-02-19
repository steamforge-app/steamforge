package steam

// CallbackMessage represents a Steam callback message.
type CallbackMessage struct {
	UserID    int32
	Callback  int32
	ParamPtr  uintptr
	ParamSize int32
}

// UserStatsReceived is callback ID 1101.
type UserStatsReceived struct {
	GameID uint64
	Result int32 // 1 = success
	UserID uint64
}

// UserStatsStored is callback ID 1102.
type UserStatsStored struct {
	GameID uint64
	Result int32 // 1 = success
}

// Callback IDs
const (
	CallbackUserStatsReceived = 1101
	CallbackUserStatsStored   = 1102
)

// EResult codes
const (
	ResultOK   = 1
	ResultFail = 2
)
