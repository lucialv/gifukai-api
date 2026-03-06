![Banner](/public/Banner.png)

This is a simple API with endpoints for anime gifs. The API is hosted on Vercel and the gifs are stored on Cloudflare R2 Buckets.

## How to use

To use the API, you need to fetch the gifs from the following URL:

```
https://cdn-anime.lucialv.com/pat
```

The URL above will return a random gif from the action `pat`. You can change the action to get gifs from different actions.

## Actions

| Function | Endpoint                           |
| -------- | ---------------------------------- |
| `pat`    | https://cdn-anime.lucialv.com/pat  |
| `hug`    | https://cdn-anime.lucialv.com/hug  |
| `kiss`   | https://cdn-anime.lucialv.com/kiss |

## Pairing filter

You can filter gifs by pairing type using the `pairing` query parameter. The first character represents who **does** the action:

| Pairing | Description | Example                                       |
| ------- | ----------- | --------------------------------------------- |
| `f`     | Solo girl   | https://cdn-anime.lucialv.com/sleep?pairing=f |
| `m`     | Solo boy    | https://cdn-anime.lucialv.com/sleep?pairing=m |
| `ff`    | Girl → Girl | https://cdn-anime.lucialv.com/hug?pairing=ff  |
| `mm`    | Boy → Boy   | https://cdn-anime.lucialv.com/hug?pairing=mm  |
| `fm`    | Girl → Boy  | https://cdn-anime.lucialv.com/kiss?pairing=fm |
| `mf`    | Boy → Girl  | https://cdn-anime.lucialv.com/kiss?pairing=mf |

If no pairing is specified, a random gif from any pairing will be returned.

### Example response of `GET /pat`

```json
{
  "action": "pat",
  "pairing": "mf",
  "url": "https://cdn.lucialv.com/pat/61178dc2-f339-48b1-89f6-ca8d10a4075d.gif",
  "filename": "61178dc2-f339-48b1-89f6-ca8d10a4075d.gif",
  "content_type": "image/gif",
  "size_bytes": 391483
}
```

Thank you for using the API! If you have any suggestions, please let me know.