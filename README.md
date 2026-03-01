![Banner](/public/Banner.png)

This is a simple API with endpoints for anime gifs. The API is hosted on Vercel and the gifs are stored on Cloudflare R2 Buckets.

## How to use

To use the API, you need to fetch the gifs from the following URL:

```
https://cdn-anime.lucialv.com/pat
```

The URL above will return a random gif from the action `pat`. You can change the action to get gifs from different actions.

## Actions

| Function | Endpoint                             |
| -------- | ------------------------------------ |
| `pat`    | https://cdn-anime.lucialv.com/pat  |
| `hug`    | https://cdn-anime.lucialv.com/hug  |
| `kiss`   | https://cdn-anime.lucialv.com/kiss |

## Pairing filter

You can filter gifs by pairing type using the `pairing` query parameter:

| Pairing | Description | Example                                         |
| ------- | ----------- | ----------------------------------------------- |
| `ff`    | Girl x Girl | https://cdn-anime.lucialv.com/hug?pairing=ff  |
| `mm`    | Boy x Boy   | https://cdn-anime.lucialv.com/hug?pairing=mm  |
| `fm`    | Girl x Boy  | https://cdn-anime.lucialv.com/kiss?pairing=fm |

If no pairing is specified, a random gif from any pairing will be returned.

### Example response of `GET /pat`

```json
{
  "action": "pat",
  "pairing": "fm",
  "url": "https://cdn.lucialv.com/pat/61178dc2-f339-48b1-89f6-ca8d10a4075d.gif",
  "filename": "61178dc2-f339-48b1-89f6-ca8d10a4075d.gif",
  "content_type": "image/gif",
  "size_bytes": 391483
}
```

Thank you for using the API! If you have any suggestions, please let me know.
