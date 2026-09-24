# model

The plain values every ring shares. It imports the standard library only.

- `Project` and its sections are the loaded config with every default applied. `adapters/config` builds it once
- `Service`, `Tier` and `Snapshot` are the runtime read model. `app/registry` holds the one copy and changes it
- `Config` is what loading the config produced: the paths, the project, its topology and the error
- `Report` is the doctor output

## Changing it

- one type per object. A runtime fact lives on the object. The process facts sit under `Service.Process`
- a derived value is a method (`Counts`, `Tally`, `DefaultOnly`), never a stored field
- a service crosses a package boundary by `ID`. `Project.Service(name)` is the one lookup by name
- `Status` and `Phase` values are in the REST API, so a shipped string is frozen
