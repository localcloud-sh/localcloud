// cmd/localcloud/templates/chat/backend/lib/stream.js

// SSE (Server-Sent Events) helper for streaming responses
export class SSEStream {
    constructor(response) {
        this.response = response
        this.initialized = false
    }

    // Initialize SSE headers
    init() {
        if (this.initialized) return

        this.response.writeHead(200, {
            'Content-Type': 'text/event-stream',
            'Cache-Control': 'no-cache',
            'Connection': 'keep-alive',
            'X-Accel-Buffering': 'no' // Disable Nginx buffering
        })

        this.initialized = true
    }

    // Send data event
    send(data) {
        this.init()

        const jsonData = typeof data === 'string' ? data : JSON.stringify(data)
        this.response.write(`data: ${jsonData}\n\n`)
    }

    // Send error event
    error(error) {
        this.init()

        const errorData = {
            error: true,
            message: error.message || 'Unknown error'
        }

        this.response.write(`data: ${JSON.stringify(errorData)}\n\n`)
    }

    // Send completion signal
    done() {
        this.init()
        this.response.write('data: [DONE]\n\n')
    }

    // End the stream
    end() {
        this.response.end()
    }
}

// Parse streaming response from Ollama
export function parseOllamaStream(stream, onChunk, onError) {
    let buffer = ''

    stream.on('data', (chunk) => {
        buffer += chunk.toString()
        const lines = buffer.split('\n')

        // Keep the last incomplete line in buffer
        buffer = lines.pop() || ''

        for (const line of lines) {
            if (line.trim()) {
                try {
                    const data = JSON.parse(line)
                    onChunk(data)
                } catch (error) {
                    // Ignore parse errors for incomplete chunks
                    console.debug('Parse error for line:', line)
                }
            }
        }
    })

    stream.on('end', () => {
        // Process any remaining data
        if (buffer.trim()) {
            try {
                const data = JSON.parse(buffer)
                onChunk(data)
            } catch (error) {
                console.debug('Parse error for remaining buffer:', buffer)
            }
        }
    })

    stream.on('error', onError)
}

// Streaming response helper for chat completions
export async function streamChatCompletion(ollamaResponse, sseStream, options = {}) {
    const {
        onToken = () => {},
        onComplete = () => {},
        includeMetadata = false
    } = options

    let fullResponse = ''
    let tokenCount = 0

    return new Promise((resolve, reject) => {
        parseOllamaStream(
            ollamaResponse.data,
            (data) => {
                if (data.message?.content) {
                    const content = data.message.content
                    fullResponse += content
                    tokenCount++

                    // Send content to client
                    sseStream.send({ content })

                    // Callback for token processing
                    onToken(content, fullResponse)
                }

                // Include metadata if requested
                if (includeMetadata && data.done) {
                    sseStream.send({
                        metadata: {
                            model: data.model,
                            total_duration: data.total_duration,
                            eval_count: data.eval_count,
                            eval_duration: data.eval_duration
                        }
                    })
                }
            },
            (error) => {
                sseStream.error(error)
                reject(error)
            }
        )

        ollamaResponse.data.on('end', () => {
            sseStream.done()
            onComplete(fullResponse, tokenCount)
            resolve(fullResponse)
        })
    })
}