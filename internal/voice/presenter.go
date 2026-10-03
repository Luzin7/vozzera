package voice

type TokenResponse struct {
	Token    string `json:"token"`
	URL      string `json:"url"`
	RoomName string `json:"room_name"`
}

func TokenPresenter(out TokenOutput) TokenResponse {
	return TokenResponse{
		Token:    out.Token,
		URL:      out.URL,
		RoomName: out.RoomName,
	}
}
