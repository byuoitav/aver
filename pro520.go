package aver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"io/ioutil"
	"net/http"
	"time"

	_ "image/jpeg"

	"github.com/byuoitav/visca"
)

type Pro520 struct {
	*visca.Camera

	Address  string
	Username string
	Password string
}

type pro520Login struct {
	Name     string `json:"name"`
	Password string `json:"password"`
}

type pro520Token struct {
	Data struct {
		Token string `json:"token"`
	} `json:"data"`
}

func (c *Pro520) Stream(ctx context.Context) (chan image.Image, chan error, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to login to camera: %w", err)
	}

	images := make(chan image.Image)
	errs := make(chan error)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				image, err := c.getLiveImage(ctx, tok)
				if err != nil {
					errs <- err
					continue
				}

				images <- image
			case <-ctx.Done():
				return
			}
		}
	}()

	return images, errs, nil
}

func (c *Pro520) getLiveImage(ctx context.Context, token string) (image.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s:81/live", c.Address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to build request: %w", err)
	}

	req.Header.Add("Authorization", "bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	image, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}

	return image, nil
}

func (c *Pro520) getToken(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	reqBody, err := json.Marshal(pro520Login{Name: c.Username, Password: c.Password})
	if err != nil {
		return "", fmt.Errorf("unable to marshal request: %w", err)
	}

	url := fmt.Sprintf("http://%s/login_name", c.Address)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("unable to build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("unable to read response: %w", err)
	}

	var tok pro520Token
	if err := json.Unmarshal(respBody, &tok); err != nil {
		return "", fmt.Errorf("unable to unmarshal response: %w", err)
	}

	return tok.Data.Token, nil
}
