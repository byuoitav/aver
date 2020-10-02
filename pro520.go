package aver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/byuoitav/visca"
)

type Pro520 struct {
	*visca.Camera

	Address  string
	Username string
	Password string
}

func (c *Pro520) RemoteAddr() string {
	return c.Address
}

func (c *Pro520) TiltUp(ctx context.Context) error {
	return c.Camera.TiltUp(ctx, 0x0e)
}

func (c *Pro520) TiltDown(ctx context.Context) error {
	return c.Camera.TiltDown(ctx, 0x0e)
}

func (c *Pro520) PanLeft(ctx context.Context) error {
	return c.Camera.PanLeft(ctx, 0x0b)
}

func (c *Pro520) PanRight(ctx context.Context) error {
	return c.Camera.PanRight(ctx, 0x0b)
}

func (c *Pro520) GoToPreset(ctx context.Context, preset string) error {
	channel, err := strconv.Atoi(preset)
	if err != nil {
		return fmt.Errorf("unable to convert preset to channel: %w", err)
	}

	return c.Camera.MemoryRecall(ctx, byte(channel))
}

func (c *Pro520) SetPreset(ctx context.Context, preset string) error {
	channel, err := strconv.Atoi(preset)
	if err != nil {
		return fmt.Errorf("unable to convert preset to channel: %w", err)
	}

	return c.Camera.MemorySet(ctx, byte(channel))
}

func (c *Pro520) ZoomIn(ctx context.Context) error {
	return c.Camera.ZoomTele(ctx)
}

func (c *Pro520) ZoomOut(ctx context.Context) error {
	return c.Camera.ZoomWide(ctx)
}

func (c *Pro520) Stream(ctx context.Context) (chan image.Image, chan error, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to login to camera: %w", err)
	}

	images := make(chan image.Image)
	errs := make(chan error)
	go func() {
		ticker := time.NewTicker(125 * time.Millisecond)
		defer ticker.Stop()
		defer close(images)
		defer close(errs)

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

func (c *Pro520) StreamJPEG(ctx context.Context) (chan []byte, chan error, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to login to camera: %w", err)
	}

	jpegs := make(chan []byte)
	errs := make(chan error)
	go func() {
		ticker := time.NewTicker(125 * time.Millisecond)
		defer ticker.Stop()
		defer close(jpegs)
		defer close(errs)

		for {
			select {
			case <-ticker.C:
				image, err := c.getLiveJPEG(ctx, tok)
				if err != nil {
					errs <- err
					continue
				}

				jpegs <- image
			case <-ctx.Done():
				return
			}
		}
	}()

	return jpegs, errs, nil
}

func (c *Pro520) Snapshot(ctx context.Context) (image.Image, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to login to camera: %w", err)
	}

	return c.getLiveImage(ctx, tok)
}

func (c *Pro520) Preset(ctx context.Context, preset string) (image.Image, error) {
	tok, err := c.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("unable to login to camera: %w", err)
	}

	url := fmt.Sprintf("http://%s:81/preset/preset_%s.jpg", c.Address, preset)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to build request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+tok)

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

func (c *Pro520) Reboot(ctx context.Context) error {
	tok, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("unable to login to camera: %w", err)
	}

	url := fmt.Sprintf("http://%s/reboot", c.Address)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("unable to build request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+tok)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("statuscode %d", resp.StatusCode)
	}

	return nil
}

func (c *Pro520) getLiveImage(ctx context.Context, token string) (image.Image, error) {
	jpeg, err := c.getLiveJPEG(ctx, token)
	if err != nil {
		return nil, err
	}

	image, _, err := image.Decode(bytes.NewReader(jpeg))
	if err != nil {
		return nil, fmt.Errorf("unable to decode image: %w", err)
	}

	return image, nil
}

func (c *Pro520) getLiveJPEG(ctx context.Context, token string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	url := fmt.Sprintf("http://%s:81/live", c.Address)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to build request: %w", err)
	}

	req.Header.Add("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to make request: %w", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read response: %w", err)
	}

	return body, nil
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
