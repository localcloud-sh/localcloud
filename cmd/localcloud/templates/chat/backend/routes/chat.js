// cmd/localcloud/templates/chat/backend/routes/chat.js
import express from 'express'
import { v4 as uuidv4 } from 'uuid'
import { SSEStream, streamChatCompletion } from '../lib/stream.js'
import localcloud from '../lib/localcloud.js'

const router = express.Router()

// Get available models from Ollama via LocalCloud
router.get('/models', async (req, res) => {
    try {
        const localcloud = req.app.locals.localcloud
        const models = await localcloud.models.list()

        res.json({
            models: models.map(model => ({
                name: model.name,
                size: model.size,
                modified: model.modified_at
            }))
        })
    } catch (error) {
        req.app.locals.logger.error('Failed to fetch models:', error)
        res.status(500).json({ error: 'Failed to fetch models' })
    }
})

// Get conversations
router.get('/conversations', async (req, res) => {
    try {
        const db = req.app.locals.localcloud.getDatabase()
        const result = await db.query(
            'SELECT * FROM conversations ORDER BY updated_at DESC LIMIT 50'
        )

        res.json({ conversations: result.rows })
    } catch (error) {
        req.app.locals.logger.error('Failed to fetch conversations:', error)
        res.status(500).json({ error: 'Failed to fetch conversations' })
    }
})

// Create conversation
router.post('/conversations', async (req, res) => {
    try {
        const { title } = req.body
        const db = req.app.locals.localcloud.getDatabase()

        const result = await db.query(
            'INSERT INTO conversations (id, title) VALUES ($1, $2) RETURNING *',
            [uuidv4(), title || 'New Conversation']
        )

        res.json({ conversation: result.rows[0] })
    } catch (error) {
        req.app.locals.logger.error('Failed to create conversation:', error)
        res.status(500).json({ error: 'Failed to create conversation' })
    }
})

// Get messages for a conversation
router.get('/conversations/:id/messages', async (req, res) => {
    try {
        const { id } = req.params
        const db = req.app.locals.localcloud.getDatabase()

        const result = await db.query(
            'SELECT * FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC',
            [id]
        )

        res.json({ messages: result.rows })
    } catch (error) {
        req.app.locals.logger.error('Failed to fetch messages:', error)
        res.status(500).json({ error: 'Failed to fetch messages' })
    }
})

// Chat completion with streaming
router.post('/chat/completions', async (req, res) => {
    const logger = req.app.locals.logger
    const localcloud = req.app.locals.localcloud
    const db = localcloud.getDatabase()

    try {
        const { message, model, conversation_id, stream = true } = req.body

        if (!message || !model) {
            return res.status(400).json({ error: 'Message and model are required' })
        }

        let conversationId = conversation_id

        // Create conversation if not provided
        if (!conversationId) {
            const convResult = await db.query(
                'INSERT INTO conversations (id, title) VALUES ($1, $2) RETURNING id',
                [uuidv4(), message.substring(0, 50) + '...']
            )
            conversationId = convResult.rows[0].id
        }

        // Save user message
        const userMessageId = uuidv4()
        await db.query(
            'INSERT INTO messages (id, conversation_id, role, content, model) VALUES ($1, $2, $3, $4, $5)',
            [userMessageId, conversationId, 'user', message, model]
        )

        // Get conversation history for context
        const historyResult = await db.query(
            'SELECT role, content FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC LIMIT 20',
            [conversationId]
        )

        // Prepare messages for Ollama
        const messages = historyResult.rows.map(msg => ({
            role: msg.role,
            content: msg.content
        }))

        if (stream) {
            // Set up SSE stream
            const sseStream = new SSEStream(res)

            try {
                // Get streaming response from Ollama
                const ollamaResponse = await localcloud.models.chat({
                    model,
                    messages,
                    stream: true
                })

                let assistantMessage = ''
                const assistantMessageId = uuidv4()

                // Stream the response
                await streamChatCompletion(
                    ollamaResponse,
                    sseStream,
                    {
                        onComplete: async (fullResponse) => {
                            // Save assistant message
                            await db.query(
                                'INSERT INTO messages (id, conversation_id, role, content, model) VALUES ($1, $2, $3, $4, $5)',
                                [assistantMessageId, conversationId, 'assistant', fullResponse, model]
                            )

                            // Update conversation timestamp
                            await db.query(
                                'UPDATE conversations SET updated_at = NOW() WHERE id = $1',
                                [conversationId]
                            )
                        }
                    }
                )

                sseStream.end()

            } catch (error) {
                logger.error('Streaming error:', error)
                sseStream.error(error)
                sseStream.end()
            }

        } else {
            // Non-streaming response
            try {
                const response = await localcloud.models.chat({
                    model,
                    messages,
                    stream: false
                })

                const assistantMessage = response.data.message.content

                // Save assistant message
                await db.query(
                    'INSERT INTO messages (id, conversation_id, role, content, model) VALUES ($1, $2, $3, $4, $5)',
                    [uuidv4(), conversationId, 'assistant', assistantMessage, model]
                )

                // Update conversation timestamp
                await db.query(
                    'UPDATE conversations SET updated_at = NOW() WHERE id = $1',
                    [conversationId]
                )

                res.json({
                    conversation_id: conversationId,
                    message: assistantMessage
                })
            } catch (error) {
                logger.error('Chat completion error:', error)
                res.status(500).json({ error: 'Failed to generate response' })
            }
        }
    } catch (error) {
        logger.error('Chat endpoint error:', error)
        res.status(500).json({ error: 'Internal server error' })
    }
})

// Delete conversation
router.delete('/conversations/:id', async (req, res) => {
    try {
        const { id } = req.params
        const db = req.app.locals.localcloud.getDatabase()

        // Delete messages first (foreign key constraint)
        await db.query('DELETE FROM messages WHERE conversation_id = $1', [id])

        // Delete conversation
        await db.query('DELETE FROM conversations WHERE id = $1', [id])

        res.json({ success: true })
    } catch (error) {
        req.app.locals.logger.error('Failed to delete conversation:', error)
        res.status(500).json({ error: 'Failed to delete conversation' })
    }
})

export default router