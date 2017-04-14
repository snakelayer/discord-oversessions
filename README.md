A simple Discord bot, showing player stats after a session of Overwatch.

Forked from [owbot-bot](https://github.com/verath/owbot-bot).
Written in go. Discord client uses [bwmarrin/discordgo](https://github.com/bwmarrin/discordgo).
Overwatch stats from [SunDwarf/OWAPI](https://github.com/SunDwarf/OWAPI).

## Running the bot
First install:

```
go get github.com/snakelayer/discord-oversessions
go install github.com/snakelayer/discord-oversessions
```

Then run it, supplying a Discord Bot ID and a Bot Token:

```
owbot-bot -token "BOT_TOKEN"
```

## Adding the bot to a channel
The bot can be added to a channel by using the Discord OAuth flow
with the `READ_MESSAGES` and `SEND_MESSAGES` permissions:

https://discordapp.com/oauth2/authorize?scope=bot&permissions=3072&client_id=CLIENT_ID

Note that CLIENT_ID is the Discord Client/Application ID, and not the Bot ID.
