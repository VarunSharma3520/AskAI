package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/parakeet-nest/parakeet/completion"
	"github.com/parakeet-nest/parakeet/enums/option"
	pkllm "github.com/parakeet-nest/parakeet/llm"

	"github.com/VarunSharma3520/AskAI/internal/types"
)

// StartStreamCmd launches a Parakeet ChatStream and emits ui messages.
// It also collects the full response and calls the provided callback with it.
func StartStreamCmd(apiURL, modelName, prompt string, temp float64,
	out chan<- string, errCh chan<- error, stopCh <-chan struct{},
) tea.Cmd {
	return startStreamCmdWithCallback(apiURL, modelName, prompt, temp, out, errCh, stopCh, nil)
}

// StartStreamCmdWithCallback launches a Parakeet ChatStream and calls the provided callback with the full response.
func StartStreamCmdWithCallback(apiURL, modelName, prompt string, temp float64,
	out chan<- string, errCh chan<- error, stopCh <-chan struct{}, responseCh chan<- string,
) tea.Cmd {
	return startStreamCmdWithCallback(apiURL, modelName, prompt, temp, out, errCh, stopCh, responseCh)
}

func startStreamCmdWithCallback(apiURL, modelName, prompt string, temp float64,
	out chan<- string, errCh chan<- error, stopCh <-chan struct{}, responseCh chan<- string,
) tea.Cmd {
	// log.Printf("Starting stream with model: %s, temperature: %.2f, prompt length: %d", modelName, temp, len(prompt))

	// Create a context that will be canceled when the stream is stopped
	ctx, cancel := context.WithCancel(context.Background())

	opts := pkllm.SetOptions(map[string]interface{}{
		string(option.Temperature): temp,
	})

	return func() tea.Msg {
		go func() {
			var fullResponse strings.Builder

			// Handle panics and cleanup
			var responseSent bool
			var responseMutex sync.Mutex

			defer func() {
				cancel() // Cancel the context when we're done

				if r := recover(); r != nil {
					// errMsg := fmt.Sprintf("Stream panic: %v", r)
					// log.Printf("ERROR: %s", errMsg)
					// If we have an error channel, send the error if we haven't already
					responseMutex.Lock()
					defer responseMutex.Unlock()

					if errCh != nil && !responseSent {
						responseSent = true
						select {
						case errCh <- fmt.Errorf("stream panic: %v", r):
							// log.Println("Sent panic to error channel")
						default: // Don't block if channel is full
							// log.Println("Error channel blocked, could not send panic")
						}
					}
				} else {
					// log.Println("Stream completed successfully")
				}

				// Close response channel if it exists and we haven't closed it yet
				if responseCh != nil {
					responseMutex.Lock()
					defer responseMutex.Unlock()
					if !responseSent {
						close(responseCh)
						responseSent = true
					}
				}
			}()

			q := pkllm.Query{
				Model: modelName,
				Messages: []pkllm.Message{
					{Role: "user", Content: prompt},
				},
				Options: opts,
				Stream:  true,
			}

			// log.Println("Starting ChatStream...")
			// Create a safe send function to prevent sending on closed channels
			safeSendString := func(ch chan<- string, data string) bool {
				if ch == nil {
					return false
				}
				select {
				case ch <- data:
					return true
				case <-ctx.Done():
					// log.Println("Context done, skipping send")
					return false
				default:
					// log.Println("Channel blocked, skipping send")
					return false
				}
			}

			// log.Println("Starting ChatStream...")
			_, err := completion.ChatStream(apiURL, q, func(ans pkllm.Answer) error {
				// Check if context is done
				select {
				case <-ctx.Done():
					// log.Println("Stream canceled by context")
					return errors.New("stream canceled")
				default:
				}

				if s := ans.Message.Content; s != "" {
					// log.Printf("Received chunk, length: %d", len(s))

					// Add to full response
					fullResponse.WriteString(s)

					// Send to output channels if they're not nil
					if out != nil {
						if !safeSendString(out, s) {
							// log.Println("Failed to send chunk to output channel")
						}
					}
				}
				return nil
			})

			// Handle any errors that occurred during streaming
			if err != nil {
				// errMsg := fmt.Sprintf("Error in ChatStream: %v", err)
				// log.Printf("ERROR: %s", errMsg)
				if errCh != nil {
					select {
					case errCh <- err:
						// log.Println("Sent error to error channel")
					default:
						// log.Println("Error channel blocked, could not send error")
					}
				}
				return
			}

			// Send the full response if we have a response channel
			if responseCh != nil {
				fullResp := fullResponse.String()
				if fullResp != "" {
					if sendFullResponse(ctx, responseCh, fullResp) {
						// log.Printf("Sent full response to channel, length: %d", len(fullResp))
					} else {
						// log.Println("Failed to deliver full response to response channel")
					}
				}
			}
		}()

		return nil
	}
}

// sendFullResponse delivers the aggregated response and respects context cancellation
func sendFullResponse(ctx context.Context, ch chan<- string, response string) bool {
	if ch == nil || response == "" {
		return false
	}

	select {
	case ch <- response:
		return true
	case <-ctx.Done():
		return false
	}
}

// NextTokenCmd waits for the next token or error/end signal.
func NextTokenCmd(ch <-chan string, errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case err := <-errCh:
			if err != nil {
				// log.Printf("Received error in NextTokenCmd: %v", err)
				return types.StreamErrMsg{Err: err}
			}
			return io.EOF
		case token, ok := <-ch:
			if !ok {
				// log.Println("Token channel closed")
				return types.StreamEndMsg{}
			}
			if token == "" {
				// log.Println("Received empty token")
				return types.StreamEndMsg{}
			}
			// log.Printf("Received token, length: %d", len(token))
			return types.TokenMsg(token)
		}
	}
}
