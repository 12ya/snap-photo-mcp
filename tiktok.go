package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ─── Tool 5: tiktok_exchange_code ─────────────────────────────────────────────

func registerTikTokExchangeCode(s *server.MCPServer) {
	tool := mcp.NewTool("tiktok_exchange_code",
		mcp.WithDescription("One-time OAuth: exchange authorization code for access + refresh tokens"),
		mcp.WithString("code", mcp.Required(), mcp.Description("Authorization code from TikTok OAuth callback (?code=...)")),
		mcp.WithString("redirect_uri", mcp.Required(), mcp.Description("Must exactly match the redirect_uri used when requesting the code")),
		mcp.WithString("code_verifier", mcp.Description("PKCE code verifier (only needed for mobile/desktop flows)")),
	)

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		code, err := req.RequireString("code")
		if err != nil {
			return nil, err
		}
		redirectURI, err := req.RequireString("redirect_uri")
		if err != nil {
			return nil, err
		}
		codeVerifier := req.GetString("code_verifier", "")

		clientKey := os.Getenv("TIKTOK_CLIENT_KEY")
		clientSecret := os.Getenv("TIKTOK_CLIENT_SECRET")
		if clientKey == "" || clientSecret == "" {
			return nil, fmt.Errorf("set TIKTOK_CLIENT_KEY and TIKTOK_CLIENT_SECRET in .env first")
		}

		params := url.Values{
			"client_key":    {clientKey},
			"client_secret": {clientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURI},
		}
		if codeVerifier != "" {
			params.Set("code_verifier", codeVerifier)
		}

		resp, err := http.Post(
			tiktokAPI+"/v2/oauth/token/",
			"application/x-www-form-urlencoded",
			strings.NewReader(params.Encode()),
		)
		if err != nil {
			return nil, fmt.Errorf("token exchange request failed: %w", err)
		}
		defer resp.Body.Close()

		var j map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
			return nil, fmt.Errorf("parse token response: %w", err)
		}

		accessToken, _ := j["access_token"].(string)
		if accessToken == "" {
			b, _ := json.Marshal(j)
			return nil, fmt.Errorf("token exchange failed: %s", string(b))
		}

		refreshToken, _ := j["refresh_token"].(string)
		setEnvVar("TIKTOK_ACCESS_TOKEN", accessToken)
		setEnvVar("TIKTOK_REFRESH_TOKEN", refreshToken)

		expiresIn := j["expires_in"]
		refreshExpiresIn := j["refresh_expires_in"]
		scope, _ := j["scope"].(string)

		text := strings.Join([]string{
			"✓ Tokens written to .env",
			fmt.Sprintf("  access_token  : %s… (expires in %vs)", accessToken[:min(12, len(accessToken))], expiresIn),
			fmt.Sprintf("  refresh_token : %s… (expires in %vs)", refreshToken[:min(12, len(refreshToken))], refreshExpiresIn),
			fmt.Sprintf("  scope         : %s", scope),
			"",
			"Restart the MCP server for the new tokens to take effect.",
		}, "\n")

		return mcp.NewToolResultText(text), nil
	})
}

// ─── Tool 6: post_to_tiktok ───────────────────────────────────────────────────

func registerPostToTikTok(s *server.MCPServer) {
	tool := mcp.NewTool("post_to_tiktok",
		mcp.WithDescription("Upload a photo carousel to TikTok via Content Posting API v2"),
		mcp.WithString("title", mcp.Description("Post title (max 150 chars)")),
		mcp.WithString("description", mcp.Description("Caption / description (max 2200 chars)")),
		mcp.WithString("privacyLevel",
			mcp.Description("Who can see the post: PUBLIC_TO_EVERYONE, MUTUAL_FOLLOW_FRIENDS, FOLLOWER_OF_CREATOR, SELF_ONLY (default SELF_ONLY)"),
		),
		mcp.WithNumber("coverIndex", mcp.Description("Which image to use as cover, 0-based (default 0)")),
	)
	// imagePaths is an array param
	tool.InputSchema.Properties["imagePaths"] = map[string]any{
		"type":        "array",
		"description": "Absolute paths to PNG images in carousel order (2–35 images)",
		"items":       map[string]any{"type": "string"},
		"minItems":    2,
		"maxItems":    35,
	}
	tool.InputSchema.Required = append(tool.InputSchema.Required, "imagePaths")

	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		imagePaths, err := req.RequireStringSlice("imagePaths")
		if err != nil {
			return nil, err
		}
		title := req.GetString("title", "")
		description := req.GetString("description", "")
		privacyLevel := req.GetString("privacyLevel", "SELF_ONLY")
		coverIndex := req.GetInt("coverIndex", 0)

		token, err := refreshAccessToken()
		if err != nil {
			return nil, err
		}

		// Step 1: init upload
		initBody, _ := json.Marshal(map[string]any{
			"post_info": map[string]any{
				"title":             title,
				"description":       description,
				"privacy_level":     privacyLevel,
				"photo_cover_index": coverIndex,
				"disable_duet":      false,
				"disable_comment":   false,
				"disable_stitch":    false,
			},
			"source_info": map[string]any{
				"source":      "FILE_UPLOAD",
				"photo_count": len(imagePaths),
			},
			"post_mode":  "MEDIA_UPLOAD",
			"media_type": "PHOTO",
		})

		initResp, err := doJSON("POST", tiktokAPI+"/v2/post/publish/content/init/", token, initBody)
		if err != nil {
			return nil, err
		}

		errCode, _ := initResp["error"].(map[string]any)
		if code, _ := errCode["code"].(string); code != "ok" {
			b, _ := json.Marshal(errCode)
			return nil, fmt.Errorf("TikTok init failed: %s", string(b))
		}

		data := initResp["data"].(map[string]any)
		publishID := data["publish_id"].(string)

		rawURLs := data["upload_url"].([]interface{})
		uploadURLs := make([]string, len(rawURLs))
		for i, u := range rawURLs {
			uploadURLs[i] = u.(string)
		}

		// Step 2: upload each image
		for i, imgPath := range imagePaths {
			fileBytes, err := os.ReadFile(imgPath)
			if err != nil {
				return nil, fmt.Errorf("read image %d: %w", i+1, err)
			}
			putReq, _ := http.NewRequest("PUT", uploadURLs[i], bytes.NewReader(fileBytes))
			putReq.Header.Set("Content-Type", "image/png")
			putReq.Header.Set("Content-Length", fmt.Sprintf("%d", len(fileBytes)))
			putReq.Header.Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", len(fileBytes)-1, len(fileBytes)))
			putResp, err := http.DefaultClient.Do(putReq)
			if err != nil {
				return nil, fmt.Errorf("upload slide %d: %w", i+1, err)
			}
			putResp.Body.Close()
			if putResp.StatusCode >= 300 {
				return nil, fmt.Errorf("image upload failed for slide %d: HTTP %d", i+1, putResp.StatusCode)
			}
		}

		// Step 3: poll for completion
		result, err := pollPublishStatus(token, publishID)
		if err != nil {
			return nil, err
		}

		result["publish_id"] = publishID
		b, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(b)), nil
	})
}

// ─── TikTok helpers ───────────────────────────────────────────────────────────

func refreshAccessToken() (string, error) {
	clientKey := os.Getenv("TIKTOK_CLIENT_KEY")
	clientSecret := os.Getenv("TIKTOK_CLIENT_SECRET")
	refreshToken := os.Getenv("TIKTOK_REFRESH_TOKEN")
	if clientKey == "" || clientSecret == "" || refreshToken == "" {
		return "", fmt.Errorf("missing TIKTOK_CLIENT_KEY, TIKTOK_CLIENT_SECRET, or TIKTOK_REFRESH_TOKEN")
	}

	params := url.Values{
		"client_key":    {clientKey},
		"client_secret": {clientSecret},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}

	resp, err := http.Post(tiktokAPI+"/v2/oauth/token/", "application/x-www-form-urlencoded", strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("token refresh request failed: %w", err)
	}
	defer resp.Body.Close()

	var j map[string]any
	json.NewDecoder(resp.Body).Decode(&j)

	token, _ := j["access_token"].(string)
	if token == "" {
		b, _ := json.Marshal(j)
		return "", fmt.Errorf("TikTok token refresh failed: %s", string(b))
	}
	return token, nil
}

func pollPublishStatus(token, publishID string) (map[string]any, error) {
	const maxAttempts = 6
	const interval = 10 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		time.Sleep(interval)

		body, _ := json.Marshal(map[string]string{"publish_id": publishID})
		result, err := doJSON("POST", tiktokAPI+"/v2/post/publish/status/fetch/", token, body)
		if err != nil {
			return nil, err
		}

		data, _ := result["data"].(map[string]any)
		status, _ := data["status"].(string)

		switch status {
		case "PUBLISH_COMPLETE":
			return data, nil
		case "FAILED":
			b, _ := json.Marshal(data)
			return nil, fmt.Errorf("TikTok publish failed: %s", string(b))
		}
		// PROCESSING_UPLOAD / PROCESSING_DOWNLOAD → keep polling
	}

	return nil, fmt.Errorf("TikTok publish timed out after %ds (publish_id: %s)",
		int(maxAttempts*interval.Seconds()), publishID)
}

func doJSON(method, url, token string, body []byte) (map[string]any, error) {
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	return result, nil
}

// ─── .env writer ──────────────────────────────────────────────────────────────

func setEnvVar(key, value string) {
	content := ""
	if b, err := os.ReadFile(envPath); err == nil {
		content = string(b)
	}

	pattern := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(key) + `=.*$`)
	line := key + "=" + value

	if pattern.MatchString(content) {
		content = pattern.ReplaceAllString(content, line)
	} else {
		if !strings.HasSuffix(content, "\n") && content != "" {
			content += "\n"
		}
		content += line + "\n"
	}
	os.WriteFile(envPath, []byte(content), 0644)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
