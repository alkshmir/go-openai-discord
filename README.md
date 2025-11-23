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

This prject uses `goreleaser` for binary and docker image distribution.

To build the image and binary locally, run the following command:

```
go tool goreleaser release --snapshot --clean
```

Note that this will produce `latest-<architecture>` tags instead of `latest` due to current limtations of goreleaser.
This behavior may change in the future.
See [discussions](https://github.com/orgs/goreleaser/discussions/6005#discussioncomment-14540712) for more details.

Set [environment variable](#deploy) to run the built container.
