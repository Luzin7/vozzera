package auth

type RegisterResponse struct {
	Message string `json:"message"`
	ID      string `json:"id"`
}

func RegisterPresenter(out RegisterOutput) RegisterResponse {
	return RegisterResponse{
		Message: "Usuário criado",
		ID:      out.ID.String(),
	}
}

type LoginResponse struct {
	Message  string `json:"message"`
	ID       string `json:"id"`
	Username string `json:"username"`
}

func LoginPresenter(out LoginOutput) LoginResponse {
	return LoginResponse{
		Message:  "Login realizado com sucesso",
		ID:       out.UserID.String(),
		Username: out.Username,
	}
}

type MeResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func MePresenter(out MeOutput) MeResponse {
	return MeResponse{
		ID:       out.ID.String(),
		Username: out.Username,
		Role:     out.Role,
	}
}
