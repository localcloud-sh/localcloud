// cmd/localcloud/templates/chat/backend/server.js
import express from 'express'
import cors from 'cors'
import { createServer } from 'http'
import rateLimit from 'express-rate-limit'
import winston from 'winston'
import { fileURLToPath } from 'url'
import { dirname } from 'path'

// Import LocalCloud SDK
import localcloud from './lib/localcloud.js'

// Import routes
import chatRoutes from './routes/chat.js'

// Setup dirname for ES modules
const __filename = fileURLToPath(import.meta.url)
const __dirname = dirname(__filename)

// Create Express app
const app = express()
const server = createServer(app)

// Logger setup
const logger = winston.createLogger({
    level: process.env.LOG_LEVEL || 'info',
    format: winston.format.combine(
        winston.format.timestamp(),
        winston.format.json()
    ),
    transports: [
        new winston.transports.Console({
            format: winston.format.simple()
        })
    ]
})

// Make logger globally available
app.locals.logger = logger
app.locals.localcloud = localcloud

// Middleware
app.use(cors({
    origin: process.env.FRONTEND_URL || `http://localhost:${process.env.FRONTEND_PORT || 3000}`,
    credentials: true
}))

app.use(express.json())
app.use(express.urlencoded({ extended: true }))

// Rate limiting
const limiter = rateLimit({
    windowMs: 1 * 60 * 1000, // 1 minute
    max: 100 // limit each IP to 100 requests per windowMs
})
app.use('/api/', limiter)

// Health check endpoint
app.get('/health', async (req, res) => {
    try {
        // Check database connection
        const db = localcloud.getDatabase()
        await db.query('SELECT 1')

        // Check Ollama connection
        const models = await localcloud.models.list()

        res.json({
            status: 'ok',
            timestamp: new Date().toISOString(),
            services: {
                database: 'connected',
                ollama: 'connected',
                models: models.length
            }
        })
    } catch (error) {
        res.status(503).json({
            status: 'error',
            timestamp: new Date().toISOString(),
            error: error.message
        })
    }
})

// API routes
app.use('/api', chatRoutes)

// Error handling middleware
app.use((err, req, res, next) => {
    logger.error('Error:', err)
    res.status(err.status || 500).json({
        error: {
            message: err.message || 'Internal server error',
            status: err.status || 500
        }
    })
})

// 404 handler
app.use((req, res) => {
    res.status(404).json({
        error: {
            message: 'Not found',
            status: 404
        }
    })
})

// Initialize database and start server
const PORT = process.env.PORT || process.env.API_PORT || 8080

async function startServer() {
    try {
        // Test database connection
        const db = localcloud.getDatabase()
        await db.query('SELECT 1')
        logger.info('Database connection established')

        // Check if required model is available
        const modelName = process.env.AI_MODEL || 'qwen2.5:3b'
        const modelExists = await localcloud.models.exists(modelName)
        if (!modelExists) {
            logger.warn(`Model ${modelName} not found. It will be pulled when needed.`)
        } else {
            logger.info(`Model ${modelName} is available`)
        }

        // Start server
        server.listen(PORT, '0.0.0.0', () => {
            logger.info(`Server running on port ${PORT}`)
            logger.info('Environment:', {
                NODE_ENV: process.env.NODE_ENV,
                DATABASE_URL: process.env.DATABASE_URL ? 'configured' : 'missing',
                OLLAMA_URL: process.env.OLLAMA_URL || 'using default'
            })
        })
    } catch (error) {
        logger.error('Failed to start server:', error)
        process.exit(1)
    }
}

// Graceful shutdown
process.on('SIGTERM', async () => {
    logger.info('SIGTERM received, shutting down gracefully...')
    server.close(async () => {
        try {
            await localcloud.cleanup()
            logger.info('Server closed')
            process.exit(0)
        } catch (error) {
            logger.error('Error during cleanup:', error)
            process.exit(1)
        }
    })
})

process.on('SIGINT', async () => {
    logger.info('SIGINT received, shutting down...')
    server.close(async () => {
        try {
            await localcloud.cleanup()
            process.exit(0)
        } catch (error) {
            process.exit(1)
        }
    })
})

// Start the server
startServer()