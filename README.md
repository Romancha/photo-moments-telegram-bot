# Photo Moments Telegram Bot

![GitHub release (with filter)](https://img.shields.io/github/v/release/Romancha/photo-moments-telegram-bot)
![GitHub Release Date - Published_At](https://img.shields.io/github/release-date/romancha/photo-moments-telegram-bot)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/Romancha/photo-moments-telegram-bot/blob/master/LICENSE)

<p align="center">
  <span>English</span> |
  <a href="https://github.com/Romancha/photo-moments-telegram-bot/tree/master/lang/ru#photo-moments-telegram-bot">Русский</a>
</p>

## Introduction

The Photo Moments Telegram Bot sends random photos from your collection to Telegram, helping you remember the good
times.

<img src="images/example_photo.jpg" width="400px">
<img src="images/example_photo_info.jpg" width="400px">

## Features

- Get random photos from your library on a schedule using [Cron](https://en.wikipedia.org/wiki/Cron).
- Request random photos by sending a message with a number or using the `/photo [count]` command. Maximum 10 photos per
  request.
- Supports various image formats: `jpg`, `png`, `gif`, `webp`, `heic`.
- Automatic compression of photos larger than 6 MB before sending.
- Show info about photo - path, time, camera model, GPS location.

## Installation and Usage

### Docker

To run from Docker, you need to do the following:

1. Install [Docker](https://docs.docker.com/get-docker/)
   and [Docker Compose](https://docs.docker.com/compose/install/).
2. Create your bot and get a token from [@BotFather](https://t.me/BotFather).
3. Get `chat_id` from [@userinfobot](https://t.me/userinfobot).
4. Set mandatory
   env [docker-compose.yml](/docker/docker-compose.yml): ``FM_ALLOWED_USERS_ID``, ``FM_CHAT_ID``, ``FM_TG_BOT_TOKEN``.
   https://github.com/Romancha/photo-moments-telegram-bot/blob/f2cf105482d3eaca339c8d0faa9292240e53c813/docker/docker-compose.yml#L1-L10
5. Configure the volumes in `docker-compose.yml` to map your photo folders.
6. Run command ``docker-compose up -d``.

You can map multiple folders with photos, for example:

```yaml
volumes:
  - /home/user/photos:/photoLibrary/
  - /home/user/photos2:/photoLibrary/
  - /home/user/photos3:/photoLibrary/
```

### Synology NAS

For Synology NAS, you can use the [Container manager](https://www.synology.com/en-us/dsm/packages/ContainerManager)
or [Docker package](https://www.synology.com/en-us/dsm/packages/Docker).

### From source

You can also run the bot from source code, build Go binary and run it. The bot requires the image process
library [libvips](https://www.libvips.org/).

## Configuration

| Param                   | Description                                                                                                |
|-------------------------|------------------------------------------------------------------------------------------------------------|
| FM_TG_BOT_TOKEN         | Telegram bot token, take from [@BotFather](https://t.me/BotFather)                                         |
| FM_CHAT_ID              | Chat ID where the bot will send messages. [@userinfobot](https://t.me/userinfobot) Can help to get chat id |
| FM_ALLOWED_USERS_ID     | Telegram user IDs that can use the bot. You can specify multiple id with separator ``;``                   |
| FM_PHOTO_PATH           | Path to the photo library folder                                                                           |
| FM_PHOTO_COUNT          | The number of photos that the bot will send according to the schedule. Default ``5``, maximum ``10``       |
| FM_SEND_PHOTO_CRON_SPEC | [Cron](https://en.wikipedia.org/wiki/Cron) to send random photos. Default ``0 10 * * *``                   |

## Commands

| Command        | Description                                                                                                |
|----------------|------------------------------------------------------------------------------------------------------------|
| [number]       | Send random photo from library. ``number`` - count of photos                                               |
| /photo [count] | Send random photo from library. ``count`` - count of photos                                                |
| /paths         | Show paths of last sent photos                                                                             |
| /info [number] | Show info about photo - path, time, camera, GPS location. ``number`` - sequence number of last sent photos |

## Contributing

We welcome contributions to improve this project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.