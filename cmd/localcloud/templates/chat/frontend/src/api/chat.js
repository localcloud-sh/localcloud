// cmd/localcloud/templates/chat/frontend/src/api/chat.js
import axios from 'axios'

const API_BASE_URL = import.meta.env.VITE_API_URL || 'http://localhost:{{.APIPort}}'

const api = axios.create({
    baseURL: API_BASE_URL,
    headers: {
        'Content-Type': 'application/json',
    },
})

export async function getModels() {
    try {
        const response = await api.get('/api/models')
        return response.data.models || []
    } catch (error) {
        console.error('Failed to fetch models:', error)
        return []
    }
}

export async function sendMessage(content, model, conversationId = null) {
    const response = await fetch(`${API_BASE_URL}/api/chat/completions`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            message: content,
            model,
            conversation_id: conversationId,
            stream: true,
        }),
    })

    if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
    }

    return response
}

export async function getConversations() {
    try {
        const response = await api.get('/api/conversations')
        return response.data.conversations || []
    } catch (error) {
        console.error('Failed to fetch conversations:', error)
        return []
    }
}

export async function getMessages(conversationId) {
    try {
        const response = await api.get(`/api/conversations/${conversationId}/messages`)
        return response.data.messages || []
    } catch (error) {
        console.error('Failed to fetch messages:', error)
        return []
    }
}

export async function createConversation(title = null) {
    try {
        const response = await api.post('/api/conversations', { title })
        return response.data.conversation
    } catch (error) {
        console.error('Failed to create conversation:', error)
        throw error
    }
}

export async function deleteConversation(conversationId) {
    try {
        await api.delete(`/api/conversations/${conversationId}`)
    } catch (error) {
        console.error('Failed to delete conversation:', error)
        throw error
    }
}