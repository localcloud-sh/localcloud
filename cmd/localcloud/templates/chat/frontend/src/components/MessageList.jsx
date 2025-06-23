// cmd/localcloud/templates/chat/frontend/src/components/MessageList.jsx
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter'
import { tomorrow } from 'react-syntax-highlighter/dist/esm/styles/prism'
import clsx from 'clsx'

export default function MessageList({ messages, isLoading }) {
    const copyToClipboard = (text) => {
        navigator.clipboard.writeText(text)
    }

    return (
        <div className="flex-1 overflow-y-auto p-4">
            <div className="max-w-4xl mx-auto space-y-4">
                {messages.length === 0 && (
                    <div className="text-center py-12">
                        <p className="text-gray-500 dark:text-gray-400 text-lg">
                            Start a conversation by typing a message below
                        </p>
                    </div>
                )}

                {messages.map((message) => (
                    <div
                        key={message.id}
                        className={clsx(
                            'flex',
                            message.role === 'user' ? 'justify-end' : 'justify-start'
                        )}
                    >
                        <div
                            className={clsx(
                                'max-w-[80%] rounded-lg px-4 py-3',
                                message.role === 'user'
                                    ? 'bg-blue-600 text-white'
                                    : message.error
                                        ? 'bg-red-100 dark:bg-red-900 text-red-900 dark:text-red-100'
                                        : 'bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100'
                            )}
                        >
                            {message.role === 'assistant' ? (
                                <ReactMarkdown
                                    remarkPlugins={[remarkGfm]}
                                    className="prose prose-sm dark:prose-invert max-w-none"
                                    components={{
                                        code({ node, inline, className, children, ...props }) {
                                            const match = /language-(\w+)/.exec(className || '')
                                            const codeString = String(children).replace(/\n$/, '')

                                            return !inline && match ? (
                                                <div className="relative group">
                                                    <button
                                                        onClick={() => copyToClipboard(codeString)}
                                                        className="absolute right-2 top-2 px-2 py-1 text-xs bg-gray-700 text-white rounded opacity-0 group-hover:opacity-100 transition-opacity"
                                                    >
                                                        Copy
                                                    </button>
                                                    <SyntaxHighlighter
                                                        style={tomorrow}
                                                        language={match[1]}
                                                        PreTag="div"
                                                        {...props}
                                                    >
                                                        {codeString}
                                                    </SyntaxHighlighter>
                                                </div>
                                            ) : (
                                                <code className={className} {...props}>
                                                    {children}
                                                </code>
                                            )
                                        }
                                    }}
                                >
                                    {message.content}
                                </ReactMarkdown>
                            ) : (
                                <p className="whitespace-pre-wrap">{message.content}</p>
                            )}

                            <div className={clsx(
                                'text-xs mt-2',
                                message.role === 'user'
                                    ? 'text-blue-200'
                                    : 'text-gray-500 dark:text-gray-400'
                            )}>
                                {new Date(message.timestamp).toLocaleTimeString()}
                            </div>
                        </div>
                    </div>
                ))}

                {isLoading && (
                    <div className="flex justify-start">
                        <div className="bg-gray-100 dark:bg-gray-800 rounded-lg px-4 py-3">
                            <div className="flex space-x-2">
                                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" />
                                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.1s' }} />
                                <div className="w-2 h-2 bg-gray-400 rounded-full animate-bounce" style={{ animationDelay: '0.2s' }} />
                            </div>
                        </div>
                    </div>
                )}
            </div>
        </div>
    )
}