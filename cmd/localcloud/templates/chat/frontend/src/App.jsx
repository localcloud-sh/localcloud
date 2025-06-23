// cmd/localcloud/templates/chat/frontend/src/App.jsx
import { useState, useEffect } from 'react'
import ChatInterface from './components/ChatInterface'
import ModelSelector from './components/ModelSelector'
import { getModels } from './api/chat'

function App() {
    const [selectedModel, setSelectedModel] = useState('{{.ModelName}}')
    const [models, setModels] = useState([])
    const [darkMode, setDarkMode] = useState(
        window.matchMedia('(prefers-color-scheme: dark)').matches
    )

    useEffect(() => {
        loadModels()
    }, [])

    useEffect(() => {
        document.documentElement.classList.toggle('dark', darkMode)
    }, [darkMode])

    const loadModels = async () => {
        try {
            const availableModels = await getModels()
            setModels(availableModels)
            if (availableModels.length > 0 && !availableModels.find(m => m.name === selectedModel)) {
                setSelectedModel(availableModels[0].name)
            }
        } catch (error) {
            console.error('Failed to load models:', error)
        }
    }

    return (
        <div className="min-h-screen bg-gray-50 dark:bg-gray-900 transition-colors">
            <header className="bg-white dark:bg-gray-800 shadow-sm border-b border-gray-200 dark:border-gray-700">
                <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                    <div className="flex items-center justify-between h-16">
                        <div className="flex items-center">
                            <h1 className="text-xl font-semibold text-gray-900 dark:text-white">
                                {{.ProjectName}}
                            </h1>
                        </div>
                        <div className="flex items-center space-x-4">
                            <ModelSelector
                                models={models}
                                selectedModel={selectedModel}
                                onModelChange={setSelectedModel}
                            />
                            <button
                                onClick={() => setDarkMode(!darkMode)}
                                className="p-2 rounded-lg hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
                                aria-label="Toggle dark mode"
                            >
                                {darkMode ? (
                                    <svg className="w-5 h-5 text-gray-600 dark:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                                    </svg>
                                ) : (
                                    <svg className="w-5 h-5 text-gray-600 dark:text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                                    </svg>
                                )}
                            </button>
                        </div>
                    </div>
                </div>
            </header>
            <main className="flex-1">
                <ChatInterface model={selectedModel} />
            </main>
        </div>
    )
}

export default App