# Photo Moments Telegram Bot

<p align="center">
  <span>English</span> |
  <a href="https://github.com/Romancha/photo-moments-telegram-bot/tree/master/lang/ru#photo-moments-telegram-bot">Русский</a>
</p>

Sends random photos from your collection to Telegram.

The idea of the bot is to get random photos from your library every day and remember the good times.

## Description

There are two triggers for sending random photos:

- Scheduled [Cron](https://en.wikipedia.org/wiki/Cron)
- Request photo by message. You can send a message to the bot with a number and it will send you a random photo from the
  library in response. Maximum number of photos is 10.

At the moment, only the local photo library is supported, the path to which must be specified when starting the bot.
Supported image formats: ``jpg``, ``png``, ``gif``, ``webp``.
If the photo is larger than 6 MB, it will be compressed before sending.

## Run

### Docker

To run from Docker, you need to do the following:

1. Install [Docker](https://docs.docker.com/get-docker/)
   and [Docker Compose](https://docs.docker.com/compose/install/).
2. Create your bot and get a token from [@BotFather](https://t.me/BotFather).
3. Get `chat_id` from [@userinfobot](https://t.me/userinfobot).
4. Set mandatory
   env [docker-compose.yml](/docker/docker-compose.yml): ``FM_ALLOWED_USERS_ID``, ``FM_CHAT_ID``, ``FM_TG_BOT_TOKEN``, ``AUTOFON_API_PASSWORD``.
   https://github.com/Romancha/photo-moments-telegram-bot/blob/f2cf105482d3eaca339c8d0faa9292240e53c813/docker/docker-compose.yml#L1-L10
5. In ``volumes``first path is the path to the folder with photos.
6. Run command ``docker-compose up -d``.

You can map multiple folders with photos, for example:

```yaml
volumes:
  - /home/user/photos:/photoLibrary/
  - /home/user/photos2:/photoLibrary/
  - /home/user/photos3:/photoLibrary/
```

### Synology NAS

For Synology NAS, you can use the [Docker package](https://www.synology.com/en-us/dsm/packages/Docker).

### From source

You can also run the bot from source code, build Go binary and run it.

Bot requires the image process library [libvips](https://www.libvips.org/).

## Environment variables

| Param                   | Description                                                                                                |
|-------------------------|------------------------------------------------------------------------------------------------------------|
| FM_TG_BOT_TOKEN         | Telegram bot token, take from [@BotFather](https://t.me/BotFather)                                         |
| FM_CHAT_ID              | Chat ID where the bot will send messages. [@userinfobot](https://t.me/userinfobot) Can help to get chat id |
| FM_ALLOWED_USERS_ID     | Telegram user IDs that can use the bot. You can specify multiple id with separator ``;``                   |
| FM_PHOTO_PATH           | Path to the photo library folder                                                                           |
| FM_PHOTO_COUNT          | The number of photos that the bot will send according to the schedule. Default ``5``, maximum ``10``       |
| FM_SEND_PHOTO_CRON_SPEC | [Cron](https://en.wikipedia.org/wiki/Cron) to send random photos. Default ``0 10-21/2 * * *``              |
