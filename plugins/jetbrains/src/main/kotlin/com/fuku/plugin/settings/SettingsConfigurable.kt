package com.fuku.plugin.settings

import com.fuku.plugin.api.ApiClient
import com.fuku.plugin.api.ApiException
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.application.ModalityState
import com.intellij.openapi.options.Configurable
import com.intellij.openapi.ui.DialogPanel
import com.intellij.ui.AnimatedIcon
import com.intellij.ui.components.JBLabel
import com.intellij.ui.dsl.builder.AlignX
import com.intellij.ui.dsl.builder.bindIntText
import com.intellij.ui.dsl.builder.bindText
import com.intellij.ui.dsl.builder.panel
import java.time.Duration
import javax.swing.JButton
import javax.swing.JComponent

class SettingsConfigurable : Configurable {
  private val settings = Settings.getInstance()

  private var host = settings.host
  private var port = settings.port
  private var token = settings.token
  private var pollInterval = settings.pollInterval
  private var fukuBinaryPath = settings.fukuBinaryPath

  private var panel: DialogPanel? = null
  private var testButton: JButton? = null
  private var testStatus: JBLabel? = null

  override fun getDisplayName(): String = "Fuku"

  override fun createComponent(): JComponent {
    val statusLabel = JBLabel("")
    testStatus = statusLabel

    val component =
      panel {
        group("Connection") {
          row("Host:") {
            textField()
              .bindText(::host)
              .align(AlignX.FILL)
          }
          row("Port:") {
            intTextField(1..65535)
              .bindIntText(::port)
          }
          row("Auth Token:") {
            passwordField()
              .bindText(::token)
              .align(AlignX.FILL)
          }
          row {
            button("Test Connection") {
              testConnection()
            }.applyToComponent {
              testButton = this
            }
            cell(statusLabel)
          }
        }
        group("Polling") {
          row("Poll Interval (ms):") {
            intTextField(500..60000)
              .bindIntText(::pollInterval)
          }
        }
        group("Binary") {
          row("Binary Path:") {
            textField()
              .bindText(::fukuBinaryPath)
              .align(AlignX.FILL)
              .comment("Path to fuku binary (absolute or on PATH)")
          }
        }
      }
    panel = component
    return component
  }

  override fun isModified(): Boolean = panel?.isModified() ?: false

  override fun apply() {
    panel?.apply()
    settings.host = host
    settings.port = port
    settings.token = token
    settings.pollInterval = pollInterval
    settings.fukuBinaryPath = fukuBinaryPath
  }

  override fun reset() {
    host = settings.host
    port = settings.port
    token = settings.token
    pollInterval = settings.pollInterval
    fukuBinaryPath = settings.fukuBinaryPath
    panel?.reset()
  }

  private fun testConnection() {
    panel?.apply()

    val testHost = host
    val testPort = port
    val testToken = token

    testButton?.isEnabled = false
    testStatus?.icon = AnimatedIcon.Default()
    testStatus?.text = "Connecting..."
    testStatus?.foreground = null

    ApplicationManager.getApplication().executeOnPooledThread {
      val client =
        ApiClient(
          host = testHost,
          port = testPort,
          token = testToken,
          connectTimeout = Duration.ofSeconds(3),
          readTimeout = Duration.ofSeconds(3),
        )

      val result = client.getStatus()
      ApplicationManager.getApplication().invokeLater({
        testButton?.isEnabled = true
        testStatus?.icon = null

        if (result.isSuccess) {
          val status = result.getOrNull()
          testStatus?.text = "\u2713 Connected (${status?.profile ?: ""})"
          testStatus?.foreground = java.awt.Color(74, 222, 128)
        } else {
          val ex = result.exceptionOrNull()
          val msg =
            when {
              ex is ApiException && ex.statusCode == 401 -> "Invalid token"
              ex is ApiException && ex.statusCode > 0 -> "HTTP ${ex.statusCode}"
              else -> ex?.message ?: "Unknown error"
            }
          testStatus?.text = "\u2717 $msg"
          testStatus?.foreground = java.awt.Color(248, 113, 113)
        }
      }, ModalityState.any())
    }
  }
}
