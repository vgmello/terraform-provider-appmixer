package client

import "context"

type loginResponse struct {
	Token string `json:"token"`
}

func (c *Client) Login(ctx context.Context, username, password string) error {
	body := map[string]string{"username": username, "password": password}
	resp, err := Post[loginResponse](ctx, c, "/user/auth", body)
	if err != nil {
		return err
	}
	c.Token = resp.Token
	return nil
}
