# Photo Moments Telegram Bot

![GitHub release (with filter)](https://img.shields.io/github/v/release/Romancha/photo-moments-telegram-bot)
![GitHub Release Date - Published_At](https://img.shields.io/github/release-date/romancha/photo-moments-telegram-bot)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](https://github.com/Romancha/photo-moments-telegram-bot/blob/master/LICENSE)

<p align="center">
  <span>English</span> |
  <a href="https://github.com/Romancha/photo-moments-telegram-bot/tree/master/lang/ru#photo-moments-telegram-bot">Русский</a>
</p>

## Introduction

The Photo Moments Telegram Bot delivers serendipitous snapshots from your personal collection straight to Telegram,
enabling you to fondly reminisce about cherished memories.

<img src="images/example_photo.jpg" width="400px">
<img src="images/example_photo_info.jpg" width="400px">

## Features

- **Random Photos on Schedule**: Fetch random photos from your library using [Cron](https://en.wikipedia.org/wiki/Cron).
- **On-Demand Random Photos**: Request random photos by sending a message with a number or using the `/photo [count]`
  command. Maximum 10 photos per request.
- **Memories from the Past**: View photos taken on this day in previous years with `/memories [years]` or `/today`.
- **Automated Memories**: Receive photos taken on this day in previous years automatically on schedule.
- **Automatic Reindexing**: Weekly differential reindexing to keep the photo database up-to-date.
- **Broad Image Format Support**: `jpg`, `png`, `gif`, `webp`, `heic`.
- **Automatic Compression** of large photos (>6 MB) before sending.
- **Detailed Photo Info**: EXIF-based details (path, date, camera model, GPS location) via `/info`.

## Installation and Usage

### Docker

1. Install [Docker](https://docs.docker.com/get-docker/) and [Docker Compose](https://docs.docker.com/compose/install/).
2. Create your bot and get a token from [@BotFather](https://t.me/BotFather).
3. Get `chat_id` from [@userinfobot](https://t.me/userinfobot).
4. Set mandatory
   env [docker-compose.yml](/docker/docker-compose.yml): ``FM_ALLOWED_USERS_ID``, ``FM_CHAT_ID``, ``FM_TG_BOT_TOKEN``,
   ``FM_DB_PATH``.
   https://github.com/Romancha/photo-moments-telegram-bot/blob/4ddf78c5b379473ae55b2b0327405199de3b0d81/docker/docker-compose.yml#L1-L11
5. Configure the volumes in `docker-compose.yml` to map your photo folders.
6. Run `docker-compose up -d`.

Example multi-folder volume mapping:

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

| Param                    | Description                                                                                                                            |
|--------------------------|----------------------------------------------------------------------------------------------------------------------------------------|
| FM_TG_BOT_TOKEN          | Telegram bot token, take from [@BotFather](https://t.me/BotFather)                                                                     |
| FM_CHAT_ID               | Chat ID where the bot will send messages. [@userinfobot](https://t.me/userinfobot) Can help to get chat id                             |
| FM_ALLOWED_USERS_ID      | Telegram user IDs that can use the bot. You can specify multiple id with separator ``;``                                               |
| FM_PHOTO_PATH            | Path to the photo library folder                                                                                                       |
| FM_DB_PATH               | Path to the db file. Default ``photo_moments.db``.                                                                                     |
| FM_PHOTO_COUNT           | The number of photos that the bot will send according to the schedule. Default ``5``, maximum ``10``                                   |
| FM_SEND_PHOTOS_BY_NUMBER | Send photos by number. Default ``true``                                                                                                |
| FM_SEND_PHOTO_CRON_SPEC  | [Cron](https://en.wikipedia.org/wiki/Cron) to send random photos. Default ``0 10 * * *``                                               |
| FM_MEMORIES_CRON_SPEC    | [Cron](https://en.wikipedia.org/wiki/Cron) to send photos from this day in different years. Default ``0 12 * * *``                     |
| FM_MEMORIES_PHOTO_COUNT  | Total number of photos to send for memories across all years. Default ``5``                                                            |
| FM_REINDEX_CRON_SPEC     | [Cron](https://en.wikipedia.org/wiki/Cron) for automatic differential reindexing. Default ``0 0 * * 0`` (weekly on Sunday at midnight) |

## Commands

| Command        | Description                                                                                                |
|----------------|------------------------------------------------------------------------------------------------------------|
| /start         | Start interacting with the bot                                                                             |
| /help          | Show help information                                                                                      |
| /random        | Get a random photo from the archive                                                                        |
| /random N      | Get N random photos from the archive                                                                       |
| /memories      | Get photos taken on this day one year ago                                                                  |
| /memories N    | Get photos taken on this day N years ago                                                                   |
| /today         | Get photos taken on this day across different years                                                        |
| /indexing      | Show the current status of photo metadata indexing                                                         |
| /reindex full  | Start full reindexing of photos (clear and recreate indices)                                               |
| /reindex diff  | Start differential indexing (only new and modified files)                                                  |
| /info [number] | Show info about photo - path, time, camera, GPS location. ``number`` - sequence number of last sent photos |
| /info          | If replying to a specific photo, shows info about that exact photo                                         |

## Contributing

We welcome contributions to improve this project.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.