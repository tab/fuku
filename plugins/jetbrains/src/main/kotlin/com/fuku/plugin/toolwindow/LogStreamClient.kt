package com.fuku.plugin.toolwindow

import com.fuku.plugin.PluginService
import com.fuku.plugin.PluginState
import com.fuku.plugin.PluginStateListener
import com.intellij.execution.ui.ConsoleView
import com.intellij.execution.ui.ConsoleViewContentType
import com.intellij.openapi.Disposable
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.editor.markup.TextAttributes
import com.intellij.ui.JBColor
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json
import java.awt.Color
import java.io.BufferedReader
import java.io.InputStreamReader
import java.net.StandardProtocolFamily
import java.net.UnixDomainSocketAddress
import java.nio.channels.Channels
import java.nio.channels.SocketChannel

class LogStreamClient(
  private val consoleView: ConsoleView,
) : PluginStateListener, Disposable {
  private val log = Logger.getInstance(LogStreamClient::class.java)
  private val json = Json { ignoreUnknownKeys = true }
  private val serviceContentTypes = mutableMapOf<String, ConsoleViewContentType>()

  @Volatile
  private var channel: SocketChannel? = null

  @Volatile
  private var readerThread: Thread? = null

  @Volatile
  private var currentProfile: String? = null

  private var maxNameLen = 0

  override fun onStateChanged(state: PluginState) {
    if (state.connected && state.status != null) {
      val profile = state.status.profile
      if (channel == null) {
        ApplicationManager.getApplication().executeOnPooledThread {
          connectSocket(profile)
        }
      } else if (profile != currentProfile) {
        disconnectSocket()
        ApplicationManager.getApplication().executeOnPooledThread {
          connectSocket(profile)
        }
      }
    } else if (!state.connected && channel != null) {
      disconnectSocket()
    }
  }

  @Synchronized
  private fun connectSocket(profile: String) {
    if (channel != null) return

    val socketPath = "/tmp/fuku-$profile.sock"
    var ch: SocketChannel? = null

    try {
      val addr = UnixDomainSocketAddress.of(socketPath)
      ch = SocketChannel.open(StandardProtocolFamily.UNIX)
      ch.connect(addr)
      channel = ch
      currentProfile = profile

      val output = Channels.newOutputStream(ch)
      val subscribe = "{\"type\":\"subscribe\",\"services\":[]}\n"
      output.write(subscribe.toByteArray(Charsets.UTF_8))
      output.flush()

      readerThread =
        Thread({
          readLoop(ch)
        }, "fuku-log-reader").apply {
          isDaemon = true
          start()
        }
    } catch (e: Exception) {
      log.info("Failed to connect to relay socket: $socketPath")
      try {
        ch?.close()
      } catch (_: Exception) {
      }
      channel = null
      currentProfile = null
    }
  }

  private fun readLoop(ch: SocketChannel) {
    try {
      val reader =
        BufferedReader(
          InputStreamReader(Channels.newInputStream(ch), Charsets.UTF_8),
        )
      while (!Thread.currentThread().isInterrupted) {
        val line = reader.readLine() ?: break
        processMessage(line)
      }
    } catch (e: Exception) {
      if (!Thread.currentThread().isInterrupted) {
        log.debug("Log stream ended: ${e.message}")
      }
    } finally {
      synchronized(this) {
        if (channel === ch) {
          channel = null
          currentProfile = null
        }
      }
    }
  }

  private fun processMessage(line: String) {
    try {
      val msg = json.decodeFromString<LogEntry>(line)
      if (msg.type != "log") return
      printLog(msg.service, msg.message)
    } catch (e: Exception) {
      log.debug("Failed to parse log message: ${e.message}")
    }
  }

  private fun printLog(
    service: String,
    message: String,
  ) {
    if (service.length > maxNameLen) maxNameLen = service.length
    val paddedName = service.padEnd(maxNameLen)

    consoleView.print(paddedName, getServiceContentType(service))
    consoleView.print(" | ", SEPARATOR_CONTENT_TYPE)
    consoleView.print("$message\n", ConsoleViewContentType.NORMAL_OUTPUT)
  }

  private fun getServiceContentType(service: String): ConsoleViewContentType {
    return serviceContentTypes.getOrPut(service) {
      val colorIndex = hashString(service) % PALETTE.size
      val color = PALETTE[colorIndex]
      val attrs = TextAttributes().apply { foregroundColor = color }
      ConsoleViewContentType("FUKU_SERVICE_$service", attrs)
    }
  }

  @Synchronized
  private fun disconnectSocket() {
    readerThread?.interrupt()
    readerThread = null
    try {
      channel?.close()
    } catch (_: Exception) {
    }
    channel = null
    currentProfile = null
    maxNameLen = 0
  }

  override fun dispose() {
    PluginService.getInstance().removeListener(this)
    disconnectSocket()
  }

  @Serializable
  private data class LogEntry(
    val type: String,
    val service: String,
    val message: String,
  )

  companion object {
    private val SEPARATOR_CONTENT_TYPE =
      ConsoleViewContentType(
        "FUKU_SEPARATOR",
        TextAttributes().apply {
          foregroundColor = JBColor(Color(115, 115, 115), Color(163, 163, 163))
        },
      )

    // 24-color palette matching fuku TUI (light/dark pairs)
    private val PALETTE: List<Color> =
      listOf(
        JBColor(Color(8, 145, 178), Color(34, 211, 238)),
        JBColor(Color(217, 119, 6), Color(251, 191, 36)),
        JBColor(Color(5, 150, 105), Color(52, 211, 153)),
        JBColor(Color(124, 58, 237), Color(167, 139, 250)),
        JBColor(Color(219, 39, 119), Color(244, 114, 182)),
        JBColor(Color(37, 99, 235), Color(96, 165, 250)),
        JBColor(Color(220, 38, 38), Color(248, 113, 113)),
        JBColor(Color(101, 163, 13), Color(163, 230, 53)),
        JBColor(Color(13, 148, 136), Color(45, 212, 191)),
        JBColor(Color(234, 88, 12), Color(251, 146, 60)),
        JBColor(Color(79, 70, 229), Color(129, 140, 248)),
        JBColor(Color(192, 38, 211), Color(232, 121, 249)),
        JBColor(Color(2, 132, 199), Color(56, 189, 248)),
        JBColor(Color(225, 29, 72), Color(251, 113, 133)),
        JBColor(Color(22, 163, 74), Color(74, 222, 128)),
        JBColor(Color(147, 51, 234), Color(192, 132, 252)),
        JBColor(Color(202, 138, 4), Color(250, 204, 21)),
        JBColor(Color(14, 116, 144), Color(103, 232, 249)),
        JBColor(Color(109, 40, 217), Color(196, 181, 253)),
        JBColor(Color(4, 120, 87), Color(110, 231, 183)),
        JBColor(Color(190, 24, 93), Color(249, 168, 212)),
        JBColor(Color(29, 78, 216), Color(147, 197, 253)),
        JBColor(Color(180, 83, 9), Color(252, 211, 77)),
        JBColor(Color(15, 118, 110), Color(94, 234, 212)),
      )

    private fun hashString(s: String): Int {
      var h = 0UL
      for (c in s) {
        h = 31UL * h + c.code.toULong()
      }
      return (h and 0x7fffffffffffffffUL).toInt()
    }
  }
}
