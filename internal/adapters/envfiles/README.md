# adapters/envfiles

Reads one `.env` file from a service directory. `*envfiles.Reader` is bound to `environment.Reader` in `bootstrap/modules/run.go`.
Which files load, their precedence and the reloads stay in `app/environment`. It skips a file on any read error.

## How it works

- `Read(dir, name)` fails with `ErrUnsafePath` on an empty name, an absolute name or a name that leaves `dir` after cleaning. It opens nothing then
- blank lines and `#` comments are skipped. A leading `export ` is dropped
- the key is the trimmed text before the first `=`. A line without a key is skipped
- the value is the rest of the line without trailing space. Quotes stay. Nothing is expanded
- entries come back in file order, duplicates included

## Changing it

- the path check stays lexical and runs before the open
- a parse rule never picks a winner between two entries. `app/environment` merges
