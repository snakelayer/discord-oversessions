A simple Discord bot, showing player stats after a session of Overwatch.

Forked from [owbot-bot](https://github.com/verath/owbot-bot). Discord client uses [bwmarrin/discordgo](https://github.com/bwmarrin/discordgo). Overwatch stats from [SunDwarf/OWAPI](https://github.com/SunDwarf/OWAPI).

## Running the bot
First install:

```
go get github.com/snakelayer/discord-oversessions
go install github.com/snakelayer/discord-oversessions
```

Then run it, supplying a Discord Bot Token and battleTagFile:

```
discord-oversessions -token "BOT_TOKEN" -battleTags <battleTagFile>
```

The battleTag file is a flat text file of records mapping discord userIds to battleTags. Each record is a single line consisting of the userId, a space, and the battle tag, eg:

```
1234567890 player#1234
```

## Adding the bot to a channel
The bot can be added to a channel by using the Discord OAuth flow
with the `READ_MESSAGES` and `SEND_MESSAGES` permissions:

https://discordapp.com/oauth2/authorize?scope=bot&permissions=3072&client_id=CLIENT_ID

Note that CLIENT_ID is the Discord Client/Application ID, and not the Bot ID.
