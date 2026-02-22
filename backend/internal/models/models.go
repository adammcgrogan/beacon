package models

type ServerStats struct {
	Players    int          `json:"players"`
	MaxPlayers int          `json:"max_players"`
	TPS        string       `json:"tps"`
	RamUsed    int64        `json:"ram_used"`
	RamMax     int64        `json:"ram_max"`
	PlayerList []PlayerInfo `json:"player_list"`
}

type PlayerInfo struct {
	Name      string `json:"name"`
	UUID      string `json:"uuid"`
	Ping      int    `json:"ping"`
	FirstJoin int64  `json:"first_join"`
	Playtime  int    `json:"playtime"`
	World     string `json:"world"`
}
