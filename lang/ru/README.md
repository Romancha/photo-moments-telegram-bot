# Photo Moments Telegram Bot

<p align="center">
  <a href="https://github.com/Romancha/photo-moments-telegram-bot#photo-moments-telegram-bot">English</a> |
  <span>Русский</span>
</p>

Отправляет случайные фотографии из вашей коллекции в Telegram.

Идея бота заключается в том, чтобы каждый день получать случайные фотографии из своей библиотеки, и вспоминать приятные
моменты.

## Описание

Отправка случайных фотографии может происходить по двум триггерам:

- По расписанию [Cron](https://en.wikipedia.org/wiki/Cron)
- По сообщению боту. Вы можете отправить боту сообщению с числом и он отправит вам случайные фотографию из библиотеки в
  ответ. Максимальное количество фотографий за один раз - 10.

На данный момент поддерживается только локальная библиотека фотографий, путь до которой нужно указать при запуске бота.
Бот умеет работать с следующими форматами: ``jpg``, ``png``, ``gif``, ``webp``.
Если фотография больше 6 Мб, то перед отправкой она будет сжата.

## Запуск

### Docker

Для запуска бота через docker compose необходимо:

1. Установить [Docker](https://docs.docker.com/get-docker/)
   и [Docker Compose](https://docs.docker.com/compose/install/).
2. Создать вашего бота и получить токен у [@BotFather](https://t.me/BotFather).
3. Узнать свой `chat_id` у [@userinfobot](https://t.me/userinfobot).
4. Заполнить [docker-compose.yml](/docker/docker-compose.yml) файл обязательными
   переменными ``FM_ALLOWED_USERS_ID``, ``FM_CHAT_ID``, ``FM_TG_BOT_TOKEN``, ``AUTOFON_API_PASSWORD``.
   https://github.com/Romancha/photo-moments-telegram-bot/blob/f2cf105482d3eaca339c8d0faa9292240e53c813/docker/docker-compose.yml#L1-L10
5. В ``volumes`` первый путь должен быть указан к папке на вашем устройстве c библиотекой фотографий.
6. Выполнить команду для запуска ``docker-compose up -d``.

Вы можете указать несколько папок с фотографиями, для этого вам нужно добавить несколько строк в ``volumes``:

```yaml
volumes:
  - /home/user/photos:/photoLibrary/
  - /home/user/photos2:/photoLibrary/
  - /home/user/photos3:/photoLibrary/
```

### Synology NAS

Для запуска бота на Synology NAS можно использовать [Docker](https://www.synology.com/en-global/dsm/packages/Docker).

### Запуск из исходников

Вы можете запустить бота, собрав его из исходников Go.

Для работы бота необходима установленная библиотека для работы с изображениями [libvips](https://www.libvips.org/).

## Доступные параметры

| Параметр                | Описание                                                                                                                              |
|-------------------------|---------------------------------------------------------------------------------------------------------------------------------------|
| FM_TG_BOT_TOKEN         | Токен телеграм бота, полученный у [@BotFather](https://t.me/BotFather)                                                                |
| FM_CHAT_ID              | Идентификатор чата, куда бот будет слать уведомления. Можно воспользоваться [@userinfobot](https://t.me/userinfobot) для получения id |
| FM_ALLOWED_USERS_ID     | Идентификаторы пользователей телеграм, которые могут пользоваться ботом. Можно указать несколько id с разделителем ``;``              |
| FM_PHOTO_PATH           | Путь до папки с библиотекой фотографий                                                                                                |
| FM_PHOTO_COUNT          | количество фотографий, которое будет отправлено ботом по расписанию. По умолчанию ``5``, максимум ``10``                              |
| FM_SEND_PHOTO_CRON_SPEC | Расписание [Cron](https://en.wikipedia.org/wiki/Cron) для отправки случайных фотографий. По умолчанию ``0 10-21/2 * * *``             |
