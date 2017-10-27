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

## Running as a Docker container
Alternatively run the bot as a docker container by cloning the repo:

```
git clone https://github.com/snakelayer/discord-oversessions.git
```

Then build and run the container:

```
docker build . -t snakelayer/discord-oversessions
docker run -d -v <VOLUME_ON_HOST>:/BattleTags snakelayer/discord-oversessions -token "BOT_TOKEN"
```

<VOLUME_ON_HOST> is the directory on the server the battletags file will be saved too, this will need creating and taggs added to before the contain will run.

## Adding the bot to a channel
The bot can be added to a channel by using the Discord OAuth flow
with the `READ_MESSAGES` and `SEND_MESSAGES` permissions:

https://discordapp.com/oauth2/authorize?scope=bot&permissions=3072&client_id=CLIENT_ID

Note that CLIENT_ID is the Discord Client/Application ID, and not the Bot ID.
