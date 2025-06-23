// cmd/localcloud/templates/chat/backend/lib/localcloud.js
import pg from 'pg'
import axios from 'axios'

const { Pool } = pg

// LocalCloud SDK wrapper for template integration
class LocalCloudSDK {
    constructor() {
        this.config = {
            databaseUrl: process.env.DATABASE_URL,
            ollamaUrl: process.env.OLLAMA_URL || 'http://localhost:11434',
            redisUrl: process.env.REDIS_URL,
            serviceDiscoveryUrl: process.env.SERVICE_DISCOVERY_URL
        }

        this._dbPool = null
        this._ollamaClient = null
    }

    // Database connection using LocalCloud-provided URL
    getDatabase() {
        if (!this._dbPool) {
            if (!this.config.databaseUrl) {
                throw new Error('DATABASE_URL not provided by LocalCloud')
            }

            this._dbPool = new Pool({
                connectionString: this.config.databaseUrl,
                max: 20,
                idleTimeoutMillis: 30000,
                connectionTimeoutMillis: 2000,
            })

            this._dbPool.on('error', (err) => {
                console.error('Unexpected error on idle database client', err)
            })
        }

        return this._dbPool
    }

    // Ollama client using LocalCloud-provided URL
    get models() {
        if (!this._ollamaClient) {
            this._ollamaClient = new OllamaClient(this.config.ollamaUrl)
        }
        return this._ollamaClient
    }

    // Service discovery helper
    async getServiceUrl(serviceName) {
        if (!this.config.serviceDiscoveryUrl) {
            // Fallback to environment variables
            switch (serviceName) {
                case 'ollama':
                    return this.config.ollamaUrl
                case 'postgres':
                    return this.config.databaseUrl
                case 'redis':
                    return this.config.redisUrl
                default:
                    throw new Error(`Unknown service: ${serviceName}`)
            }
        }

        // Use LocalCloud service discovery
        try {
            const response = await axios.get(`${this.config.serviceDiscoveryUrl}/services/${serviceName}`)
            return response.data.url
        } catch (error) {
            throw new Error(`Failed to discover service ${serviceName}: ${error.message}`)
        }
    }

    // Cleanup resources
    async cleanup() {
        if (this._dbPool) {
            await this._dbPool.end()
            this._dbPool = null
        }
    }
}

// Ollama client wrapper
class OllamaClient {
    constructor(baseUrl) {
        this.baseUrl = baseUrl
        this.axios = axios.create({
            baseURL: baseUrl,
            timeout: 300000, // 5 minutes for long generations
        })
    }

    // List available models
    async list() {
        try {
            const response = await this.axios.get('/api/tags')
            return response.data.models || []
        } catch (error) {
            console.error('Failed to list models:', error)
            return []
        }
    }

    // Chat completion with streaming support
    async chat({ model, messages, stream = true, options = {} }) {
        try {
            const response = await this.axios.post(
                '/api/chat',
                {
                    model,
                    messages,
                    stream,
                    options
                },
                {
                    responseType: stream ? 'stream' : 'json'
                }
            )

            return response
        } catch (error) {
            throw new Error(`Ollama chat failed: ${error.message}`)
        }
    }

    // Generate completion (non-chat)
    async generate({ model, prompt, stream = true, options = {} }) {
        try {
            const response = await this.axios.post(
                '/api/generate',
                {
                    model,
                    prompt,
                    stream,
                    options
                },
                {
                    responseType: stream ? 'stream' : 'json'
                }
            )

            return response
        } catch (error) {
            throw new Error(`Ollama generate failed: ${error.message}`)
        }
    }

    // Pull a model
    async pull(modelName) {
        try {
            const response = await this.axios.post(
                '/api/pull',
                { name: modelName },
                { responseType: 'stream' }
            )

            return response
        } catch (error) {
            throw new Error(`Failed to pull model ${modelName}: ${error.message}`)
        }
    }

    // Check if model exists
    async exists(modelName) {
        const models = await this.list()
        return models.some(m => m.name === modelName)
    }
}

// Export singleton instance
const localcloud = new LocalCloudSDK()

export default localcloud
export { LocalCloudSDK, OllamaClient }