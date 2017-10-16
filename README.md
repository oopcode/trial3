## USAGE

You need Docker and Docker-compose installed to run this example:

```
docker-compose build && docker-compose up
```

To install dependencies locally, run:

```
glide i --force
```

## Notes

I treated the embedded `data` field as having no strict structure, thus `data` is saved as a `json` field.
