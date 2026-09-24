# plugins/jetbrains

The IntelliJ plugin: Kotlin, Gradle, built against IDEA Community.

## Running

From the repository root:

```sh
make lint:plugin        # ktlintCheck
make lint:plugin:fix    # ktlintFormat
make build:plugin       # buildPlugin, lands in build/distributions
make clean:plugin
```

`./gradlew <task>` from this directory does the same. `pre-push` runs `make lint:plugin` when anything here moves.
`checks.yaml` runs ktlint and a build on the pull request. Neither runs the Go loop for a plugin-only change.

## Versions

`gradle.properties` holds every version: `pluginVersion` (in step with the fuku release), `platformVersion` (the IDE built against) and `sinceBuild`.
`untilBuild` stays null on purpose, so a new IDE release does not strand the plugin.
`build.gradle.kts` pins Kotlin 1.9.25 and a JVM 21 toolchain.

## Layout

- `api/` is the HTTP client and the wire models, written against `spec/openapi.yaml`. A change in the spec lands here
- `toolwindow/` is the service table and the log stream
- `statusbar/`, `settings/` and `run/` are the widget, the settings panel and the run configuration
- `src/main/resources/META-INF/plugin.xml` declares every extension point. Register a new action, tool window or service there, or it does not exist

## Conventions

- ktlint owns the formatting and skips `*.kts`. Run `make lint:plugin:fix` instead of fixing by hand
- wire models are `@Serializable` data classes and enums in `api/Models.kt`.
  They decode with `ignoreUnknownKeys`, so a new API field does not break an older plugin
- `ApiClient` defaults to `127.0.0.1:9876` and talks to `/api/v1` over `java.net.http`. It keeps no copy of fuku's state. It asks
