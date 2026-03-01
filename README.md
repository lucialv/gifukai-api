![Banner](/public/Banner.png)

This is a simple API with endpoints for anime gifs. The API is hosted on Vercel and the gifs are stored on Cloudflare R2 Buckets.

## How to use

To use the API, you need to fetch the gifs from the following URL:

```
https://cdn-anime.lucia-dev.com/pat
```

The URL above will return a random gif from the action `pat`. You can change the action to get gifs from different actions.

## Actions

| Function | Endpoint                             |
| -------- | ------------------------------------ |
| `pat`    | https://cdn-anime.lucia-dev.com/pat  |
| `hug`    | https://cdn-anime.lucia-dev.com/hug  |
| `kiss`   | https://cdn-anime.lucia-dev.com/kiss |

## Pairing filter

You can filter gifs by pairing type using the `pairing` query parameter:

| Pairing | Description | Example                                         |
| ------- | ----------- | ----------------------------------------------- |
| `ff`    | Girl x Girl | https://cdn-anime.lucia-dev.com/hug?pairing=ff  |
| `mm`    | Boy x Boy   | https://cdn-anime.lucia-dev.com/hug?pairing=mm  |
| `fm`    | Girl x Boy  | https://cdn-anime.lucia-dev.com/kiss?pairing=fm |

If no pairing is specified, a random gif from any pairing will be returned.

Thank you for using the API! If you have any suggestions, please let me know.
