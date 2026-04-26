package com.fuku.plugin.run

import com.fuku.plugin.settings.Settings
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.process.KillableProcessHandler
import com.intellij.execution.process.ProcessAdapter
import com.intellij.execution.process.ProcessEvent
import com.intellij.openapi.Disposable
import com.intellij.openapi.components.Service
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.project.Project
import java.io.File
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.atomic.AtomicBoolean

interface ProcessStateListener {
  fun onProcessStateChanged(running: Boolean)
}

@Service(Service.Level.PROJECT)
class ProcessHandler(private val project: Project) : Disposable {
  private val log = Logger.getInstance(ProcessHandler::class.java)
  private val listeners = CopyOnWriteArrayList<ProcessStateListener>()

  private val running = AtomicBoolean(false)

  val isRunning: Boolean
    get() = running.get()

  @Volatile
  private var processHandler: KillableProcessHandler? = null

  fun start(profile: String) {
    if (!running.compareAndSet(false, true)) return

    val settings = Settings.getInstance()
    val commandLine = GeneralCommandLine(settings.fukuBinaryPath, "--no-ui", "run", profile)
    commandLine.workDirectory = project.basePath?.let { File(it) }
    commandLine.charset = Charsets.UTF_8

    try {
      val handler = KillableProcessHandler(commandLine)
      handler.addProcessListener(
        object : ProcessAdapter() {
          override fun processTerminated(event: ProcessEvent) {
            running.set(false)
            processHandler = null
            notifyListeners()
          }
        },
      )

      processHandler = handler
      notifyListeners()
      handler.startNotify()
    } catch (e: Exception) {
      log.warn("Failed to start fuku process", e)
      running.set(false)
      notifyListeners()
    }
  }

  fun stop() {
    val handler = processHandler ?: return
    handler.destroyProcess()
  }

  fun readProfiles(): List<String> {
    val basePath = project.basePath ?: return listOf("default")

    val configFile =
      resolveFile(basePath, "fuku.yaml", "fuku.yml")
        ?: return listOf("default")

    val profiles = parseProfileEntries(configFile).toMutableMap()

    val overrideFile = resolveFile(basePath, "fuku.override.yaml", "fuku.override.yml")
    if (overrideFile != null) {
      for ((name, value) in parseProfileEntries(overrideFile)) {
        if (value == "null" || value == "~") {
          profiles.remove(name)
        } else {
          profiles[name] = value
        }
      }
    }

    return profiles.keys.toList().ifEmpty { listOf("default") }
  }

  private fun resolveFile(
    basePath: String,
    vararg candidates: String,
  ): File? {
    for (name in candidates) {
      val file = File(basePath, name)
      if (file.exists()) return file
    }
    return null
  }

  private fun parseProfileEntries(file: File): Map<String, String> {
    return try {
      val lines = file.readLines()
      val start = lines.indexOfFirst { it.trimStart().startsWith("profiles:") }
      if (start < 0) return emptyMap()

      val baseIndent = lines[start].indexOfFirst { !it.isWhitespace() }
      val entries = mutableMapOf<String, String>()
      for (i in (start + 1) until lines.size) {
        val line = lines[i]
        if (line.isBlank()) continue
        val trimmed = line.trimStart()
        if (trimmed.startsWith("#")) continue
        val indent = line.indexOfFirst { !it.isWhitespace() }
        if (indent <= baseIndent) break
        val colonIdx = trimmed.indexOf(':')
        if (colonIdx > 0) {
          val name = trimmed.substring(0, colonIdx).trim()
          var value = trimmed.substring(colonIdx + 1).trim()
          val commentIdx = value.indexOf(" #")
          if (commentIdx >= 0) value = value.substring(0, commentIdx).trim()
          entries[name] = value
        }
      }
      entries
    } catch (e: Exception) {
      log.info("Could not parse ${file.name} for profiles", e)
      emptyMap()
    }
  }

  fun addListener(listener: ProcessStateListener) {
    listeners.add(listener)
  }

  fun removeListener(listener: ProcessStateListener) {
    listeners.remove(listener)
  }

  private fun notifyListeners() {
    val isUp = isRunning
    for (listener in listeners) {
      try {
        listener.onProcessStateChanged(isUp)
      } catch (e: Exception) {
        log.warn("Process state listener threw exception", e)
      }
    }
  }

  override fun dispose() {
    stop()
    listeners.clear()
  }
}
