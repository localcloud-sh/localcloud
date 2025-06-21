# test_ollama.py
import requests
import json

response = requests.post('http://localhost:11434/api/generate',
json={
"model": "gemma2:2b",
"prompt": "Write a haiku about coding",
"stream": False
})

print(json.loads(response.text)['response'])