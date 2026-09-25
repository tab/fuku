# app/profiles

Turns a profile name into the ordered tiers of a run. `services.Runtime`, `services.Cleaner` and `doctor` call `Resolve` through their own interfaces.
The YAML shape of a profile and the tier order belong to `adapters/config`. They arrive here as `model.Project`.

## How it works

- an unknown profile fails with `contracts.ErrProfileNotFound`. `*` selects every service. A list selects the names it holds
- a name in the top-level `exclude` list is dropped before the lookup. An excluded unknown name is no error
- any other unknown name fails with `contracts.ErrServiceNotFound`
- the services keep the order of `Project.Services`: tier position, then name. The order of the profile list does not count
- consecutive services of one tier form one `model.Tier` with a fresh ID. An empty selection returns no tiers

## Changing it

- each tier holds a copy of the service with its `ID` and configuration. `Resolve` never changes the project
- the tier order is decided in `adapters/config`. This package reads it and never sorts
