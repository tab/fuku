package com.fuku.plugin.statusbar

import com.fuku.plugin.PluginService
import com.fuku.plugin.PluginState
import com.fuku.plugin.PluginStateListener
import com.fuku.plugin.api.Phase
import com.fuku.plugin.api.ServiceStatus
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.StatusBar
import com.intellij.openapi.wm.StatusBarWidget
import com.intellij.openapi.wm.StatusBarWidgetFactory
import com.intellij.openapi.wm.ToolWindowManager
import com.intellij.util.Consumer
import java.awt.Component
import java.awt.event.MouseEvent

private const val WIDGET_ID = "com.fuku.plugin.statusbar"

private val SPINNER = charArrayOf('\u2807', '\u2836', '\u2834', '\u2826', '\u2838', '\u2819')

class StatusBarWidgetFactory : StatusBarWidgetFactory {
  override fun getId(): String = WIDGET_ID

  override fun getDisplayName(): String = "fuku"

  override fun isAvailable(project: Project): Boolean = true

  override fun createWidget(project: Project): StatusBarWidget = StatusBarWidget(project)
}

class StatusBarWidget(
  private val project: Project,
) : StatusBarWidget, StatusBarWidget.TextPresentation, PluginStateListener {
  @Volatile
  private var text: String = "fuku: disconnected"

  @Volatile
  private var tooltip: String = "fuku service orchestrator"

  private var statusBar: StatusBar? = null
  private var spinnerIndex = 0

  private val spinnerTimer =
    javax.swing.Timer(100) {
      spinnerIndex = (spinnerIndex + 1) % SPINNER.size
      statusBar?.updateWidget(WIDGET_ID)
    }

  init {
    PluginService.getInstance().addListener(this)
  }

  override fun ID(): String = WIDGET_ID

  override fun install(statusBar: StatusBar) {
    this.statusBar = statusBar
  }

  override fun getPresentation(): StatusBarWidget.WidgetPresentation = this

  override fun getText(): String {
    val state = PluginService.getInstance().state
    if (!state.connected || state.status == null) return text

    val status = state.status
    val counts = status.services

    return when (status.phase) {
      Phase.startup -> {
        val starting = state.services.firstOrNull { it.status == ServiceStatus.starting }
        val label = if (starting != null) "starting ${starting.name}\u2026" else "starting\u2026"
        "fuku: ${SPINNER[spinnerIndex]} $label"
      }
      Phase.running -> "fuku: running ${counts.running}/${counts.total} ready"
      Phase.stopping -> "fuku: ${SPINNER[spinnerIndex]} stopping\u2026"
      Phase.stopped -> "fuku: stopped"
    }
  }

  override fun getAlignment(): Float = Component.LEFT_ALIGNMENT

  override fun getTooltipText(): String = tooltip

  override fun getClickConsumer(): Consumer<MouseEvent> =
    Consumer { _ ->
      val toolWindow = ToolWindowManager.getInstance(project).getToolWindow("fuku")
      toolWindow?.show()
    }

  override fun onStateChanged(state: PluginState) {
    ApplicationManager.getApplication().invokeLater {
      if (state.connected && state.status != null) {
        val status = state.status
        val counts = status.services

        when (status.phase) {
          Phase.startup, Phase.stopping -> {
            if (!spinnerTimer.isRunning) spinnerTimer.start()
          }
          else -> {
            if (spinnerTimer.isRunning) spinnerTimer.stop()
          }
        }

        text =
          when (status.phase) {
            Phase.startup -> "fuku: starting\u2026"
            Phase.running -> "fuku: running ${counts.running}/${counts.total} ready"
            Phase.stopping -> "fuku: stopping\u2026"
            Phase.stopped -> "fuku: stopped"
          }

        tooltip = "fuku: ${status.profile} \u2022 ${status.phase.name}"
        if (counts.failed > 0) {
          tooltip += " \u2022 ${counts.failed} failed"
        }
      } else {
        if (spinnerTimer.isRunning) spinnerTimer.stop()
        text = "fuku: disconnected"
        tooltip = "fuku service orchestrator"
      }

      statusBar?.updateWidget(WIDGET_ID)
    }
  }

  override fun dispose() {
    spinnerTimer.stop()
    PluginService.getInstance().removeListener(this)
    statusBar = null
  }
}
