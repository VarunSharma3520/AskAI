package llm

import (
	"errors"
	"strings"

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
	opts := pkllm.SetOptions(map[string]interface{}{
		string(option.Temperature): temp,
	})

	return func() tea.Msg {
		go func() {
			defer func() {
				// Recover from any panic during channel operations
				if r := recover(); r != nil {
					// Channel was closed, this is expected during cancellation
					return
				}
			}()

			// Create a local copy of channels to avoid race conditions
			localOut := out
			localErrCh := errCh
			localResponseCh := responseCh

			// Buffer to collect the full response
			var fullResponse strings.Builder

			q := pkllm.Query{
				Model: modelName,
				Messages: []pkllm.Message{
					{Role: "user", Content: prompt},
				},
				Options: opts,
				Stream:  true,
			}

			_, err := completion.ChatStream(apiURL, q, func(ans pkllm.Answer) error {
				// Check if we should stop
				select {
				case <-stopCh:
					return errors.New("stream canceled")
				default:
				}

				// Safely send to the output channel
				if s := ans.Message.Content; s != "" {
					// Add to full response
					fullResponse.WriteString(s)

					// Send to the response channel if provided
					if localResponseCh != nil {
						select {
						case <-stopCh:
							return errors.New("stream canceled")
						case localResponseCh <- s:
							// Successfully sent
						}
					}

					// Send to the output channel
					select {
					case <-stopCh:
						return errors.New("stream canceled")
					case localOut <- s:
						// Successfully sent
					}
				}
				return nil
			})

			// Send the full response to the callback channel if it exists
			if localResponseCh != nil {
				select {
				case <-stopCh:
					// Don't send if we're stopping
				default:
					sendFullResponse(localResponseCh, fullResponse.String())
				}
			}

			// Send error if any, but only if the channel is still open
			if err != nil {
				select {
				case <-stopCh:
					// Don't send error if we're stopping
				case localErrCh <- err:
					// Error sent successfully
				}
			}

			// Close the output channels
			if localOut != nil {
				close(localOut)
			}
			if localResponseCh != nil {
				close(localResponseCh)
			}
		}()
		return nil
	}
}

// sendFullResponse sends the full response to the channel in a non-blocking way
func sendFullResponse(ch chan<- string, response string) {
	go func() {
		defer func() {
			recover() // Ignore panic if channel is closed
		}()
		ch <- response
	}()
}

// NextTokenCmd waits for the next token or error/end signal.
func NextTokenCmd(ch <-chan string, errCh <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case err := <-errCh:
			if err != nil {
				return types.StreamErrMsg{Err: err}
			}
			return types.StreamEndMsg{}
		case s, ok := <-ch:
			if !ok {
				return types.StreamEndMsg{}
			}
			return types.TokenMsg(s)
		}
	}
}
