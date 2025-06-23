// cmd/localcloud/templates/chat/frontend/src/components/ChatInterface.jsx
import { useState, useRef, useEffect } from 'react'
import MessageList from './MessageList'
import { sendMessage, getConversations, getMessages } from '../api/chat'

export default function ChatInterface({ model }) {
    const [messages, setMessages] = useState([])
    const [input, setInput] = useState('')
    const [isLoading, setIsLoading] = useState(false)
    const [conversations, setConversations] = useState([])
    const [currentConversation, setCurrentConversation] = useState(null)
    const inputRef = useRef(null)
    const messagesEndRef = useRef(null)

    useEffect(() => {
        loadConversations()
    }, [])

    useEffect(() => {
        scrollToBottom()
    }, [messages])

    const loadConversations = async () => {
        try {
            const convs = await getConversations()
            setConversations(convs)
            if (convs.length > 0 && !currentConversation) {
                selectConversation(convs[0])
            }
        } catch (error) {
            console.error('Failed to load conversations:', error)
        }
    }

    const selectConversation = async (conversation) => {
        setCurrentConversation(conversation)
        try {
            const msgs = await getMessages(conversation.id)
            setMessages(msgs)
        } catch (error) {
            console.error('Failed to load messages:', error)
        }
    }

    const scrollToBottom = () => {
        messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
    }

    const handleSubmit = async (e) => {
        e.preventDefault()
        if (!input.trim() || isLoading) return

        const userMessage = {
            id: Date.now(),
            role: 'user',
            content: input.trim(),
            timestamp: new Date().toISOString()
        }

        setMessages(prev => [...prev, userMessage])
        setInput('')
        setIsLoading(true)

        try {
            const response = await sendMessage(
                input.trim(),
                model,
                currentConversation?.id
            )

            // Handle streaming response
            const reader = response.body.getReader()
            const decoder = new TextDecoder()
            let assistantMessage = {
                id: Date.now() + 1,
                role: 'assistant',
                content: '',
                timestamp: new Date().toISOString()
            }

            setMessages(prev => [...prev, assistantMessage])

            while (true) {
                const { done, value } = await reader.read()
                if (done) break

                const chunk = decoder.decode(value)
                const lines = chunk.split('\n')

                for (const line of lines) {
                    if (line.startsWith('data: ')) {
                        try {
                            const data = JSON.parse(line.slice(6))
                            if (data.content) {
                                assistantMessage.content += data.content
                                setMessages(prev =>
                                    prev.map(msg =>
                                        msg.id === assistantMessage.id
                                            ? { ...msg, content: assistantMessage.content }
                                            : msg
                                    )
                                )
                            }
                        } catch (e) {
                            // Skip invalid JSON
                        }
                    }
                }
            }

            // Reload conversations to update the list
            loadConversations()
        } catch (error) {
            console.error('Failed to send message:', error)
            setMessages(prev => [...prev, {
                id: Date.now() + 1,
                role: 'assistant',
                content: 'Sorry, I encountered an error. Please try again.',
                timestamp: new Date().toISOString(),
                error: true
            }])
        } finally {
            setIsLoading(false)
            inputRef.current?.focus()
        }
    }

    const startNewConversation = () => {
        setCurrentConversation(null)
        setMessages([])
        inputRef.current?.focus()
    }

    return (
        <div className="flex h-[calc(100vh-4rem)]">
            {/* Sidebar */}
            <div className="w-64 bg-gray-100 dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 p-4 overflow-y-auto">
                <button
                    onClick={startNewConversation}
                    className="w-full mb-4 px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
                >
                    New Chat
                </button>
                <div className="space-y-2">
                    {conversations.map(conv => (
                        <button
                            key={conv.id}
                            onClick={() => selectConversation(conv)}
                            className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                                currentConversation?.id === conv.id
                                    ? 'bg-white dark:bg-gray-700 shadow-sm'
                                    : 'hover:bg-gray-200 dark:hover:bg-gray-700'
                            }`}
                        >
                            <div className="text-sm font-medium text-gray-900 dark:text-white truncate">
                                {conv.title || 'New Conversation'}
                            </div>
                            <div className="text-xs text-gray-500 dark:text-gray-400">
                                {new Date(conv.updated_at).toLocaleDateString()}
                            </div>
                        </button>
                    ))}
                </div>
            </div>

            {/* Chat Area */}
            <div className="flex-1 flex flex-col">
                <MessageList messages={messages} isLoading={isLoading} />

                <form onSubmit={handleSubmit} className="border-t border-gray-200 dark:border-gray-700 p-4">
                    <div className="max-w-4xl mx-auto flex gap-4">
                        <input
                            ref={inputRef}
                            type="text"
                            value={input}
                            onChange={(e) => setInput(e.target.value)}
                            placeholder="Type your message..."
                            disabled={isLoading}
                            className="flex-1 px-4 py-2 border border-gray-300 dark:border-gray-600 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 dark:bg-gray-800 dark:text-white disabled:opacity-50"
                        />
                        <button
                            type="submit"
                            disabled={!input.trim() || isLoading}
                            className="px-6 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
                        >
                            Send
                        </button>
                    </div>
                </form>
                <div ref={messagesEndRef} />
            </div>
        </div>
    )
}