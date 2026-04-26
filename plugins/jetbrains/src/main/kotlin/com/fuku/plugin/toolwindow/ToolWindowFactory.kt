package com.fuku.plugin.toolwindow

import com.fuku.plugin.PluginService
import com.fuku.plugin.run.ProcessHandler
import com.intellij.execution.filters.TextConsoleBuilderFactory
import com.intellij.openapi.actionSystem.ActionManager
import com.intellij.openapi.actionSystem.ActionUpdateThread
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.CustomShortcutSet
import com.intellij.openapi.actionSystem.IdeActions
import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.content.ContentFactory
import java.awt.event.KeyEvent
import javax.swing.KeyStroke

class ToolWindowFactory : ToolWindowFactory {
  override fun createToolWindowContent(
    project: Project,
    toolWindow: ToolWindow,
  ) {
    val processHandler = project.getService(ProcessHandler::class.java)

    val consoleBuilder = TextConsoleBuilderFactory.getInstance().createBuilder(project)
    consoleBuilder.setViewer(true)
    val consoleView = consoleBuilder.console

    val logClient = LogStreamClient(consoleView)
    PluginService.getInstance().addListener(logClient)

    val panel = ServiceToolWindow(project, processHandler)

    // Bind "/" to open search in the logs console
    object : AnAction() {
      override fun actionPerformed(e: AnActionEvent) {
        ActionManager.getInstance().getAction(IdeActions.ACTION_FIND)?.actionPerformed(e)
      }

      override fun getActionUpdateThread() = ActionUpdateThread.EDT
    }.registerCustomShortcutSet(
      CustomShortcutSet(KeyStroke.getKeyStroke(KeyEvent.VK_SLASH, 0)),
      consoleView.component,
    )

    val servicesContent =
      ContentFactory.getInstance()
        .createContent(panel, "services", false)
    servicesContent.setDisposer {
      panel.dispose()
    }

    val logsContent =
      ContentFactory.getInstance()
        .createContent(consoleView.component, "logs", false)
    logsContent.setDisposer {
      logClient.dispose()
      consoleView.dispose()
    }

    toolWindow.contentManager.addContent(servicesContent)
    toolWindow.contentManager.addContent(logsContent)
  }
}
