package app

import (
	"context"
	"fmt"
	"github.com/halunhaku/tingyu/client/internal/library"
	"github.com/halunhaku/tingyu/client/internal/metadata"
	"strings"
	"time"
)

func (a *application) TestAIConnection(settings metadata.AISettings, id uint64) {
	go func() {
		message := ""
		if err := metadata.ValidateAISettings(settings); err != nil {
			message = err.Error()
		} else {
			settings.Enabled = true // Test the draft even when recognition is disabled.
			client := metadata.NewAIClient(settings, a.metadataHTTP)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			start := time.Now()
			if err := client.TestConnection(ctx); err != nil {
				message = "连接测试失败：" + metadata.AIErrorMessage(err)
			} else {
				message = fmt.Sprintf("连接成功：模型已响应（%.1f 秒）；测试未保存配置", time.Since(start).Seconds())
			}
		}
		a.updateWindow(func() { a.model.State().FinishAITest(id, message) })
	}()
}

func (a *application) RecognizeAI(track library.Track, id uint64) {
	settings := a.model.State().AI()
	go func() {
		var result *metadata.AIResult
		message := ""
		if err := metadata.ValidateAISettings(settings); err != nil {
			message = err.Error()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			input := strings.Join([]string{track.Title, track.Artist, track.Album}, " | ")
			hits, err := metadata.NewAIClient(settings, a.metadataHTTP).CleanFilenames(ctx, []metadata.AIInput{{ID: "1", Filename: input}})
			if err != nil {
				message = "AI 识别失败：" + metadata.AIErrorMessage(err)
			} else if hit, ok := hits["1"]; ok {
				result = &hit
			} else {
				message = "AI 未识别出歌曲，请修改信息后重试"
			}
		}
		a.updateWindow(func() { a.model.State().FinishAIRecognition(id, result, message) })
	}()
}
