# go-openai-discord
Golang implementation of OpenAI chatbot as discord bot

## deploy
create `.env` file and replace the placeholders with your credentials
```
OPENAI_API_KEY=<OPENAI API KEY>
DISCORD_BOT_TOKEN=<DISCORD BOT TOKEN>
```

Try your bot:
```
go run main.go
```

### Docker
Build a container:
```
docker build -t openai-discord .
```

Run the container:
```
docker run --env-file .env openai-discord
```
