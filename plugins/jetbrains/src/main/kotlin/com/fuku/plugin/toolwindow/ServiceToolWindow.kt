package com.fuku.plugin.toolwindow

import com.fuku.plugin.Icons
import com.fuku.plugin.PluginService
import com.fuku.plugin.PluginState
import com.fuku.plugin.PluginStateListener
import com.fuku.plugin.api.Phase
import com.fuku.plugin.api.Service
import com.fuku.plugin.api.ServiceStatus
import com.fuku.plugin.run.ProcessHandler
import com.fuku.plugin.run.ProcessStateListener
import com.intellij.notification.NotificationGroupManager
import com.intellij.notification.NotificationType
import com.intellij.openapi.actionSystem.ActionManager
import com.intellij.openapi.actionSystem.ActionPlaces
import com.intellij.openapi.actionSystem.ActionUpdateThread
import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.actionSystem.DefaultActionGroup
import com.intellij.openapi.actionSystem.ex.ComboBoxAction
import com.intellij.openapi.application.ApplicationManager
import com.intellij.openapi.diagnostic.Logger
import com.intellij.openapi.options.ShowSettingsUtil
import com.intellij.openapi.project.Project
import com.intellij.openapi.ui.Messages
import com.intellij.openapi.ui.SimpleToolWindowPanel
import com.intellij.ui.components.JBLabel
import com.intellij.ui.components.JBScrollPane
import com.intellij.ui.components.JBTextField
import java.awt.BorderLayout
import java.awt.event.ActionEvent
import java.awt.event.KeyAdapter
import java.awt.event.KeyEvent
import java.awt.event.MouseAdapter
import java.awt.event.MouseEvent
import javax.swing.AbstractAction
import javax.swing.BorderFactory
import javax.swing.Icon
import javax.swing.JComponent
import javax.swing.JPanel
import javax.swing.KeyStroke
import javax.swing.border.EmptyBorder
import javax.swing.event.DocumentEvent
import javax.swing.event.DocumentListener

class ServiceToolWindow(
  private val project: Project,
  private val processHandler: ProcessHandler,
) : SimpleToolWindowPanel(true, true), PluginStateListener, ProcessStateListener {
  private val log = Logger.getInstance(ServiceToolWindow::class.java)
  private val tableModel = ServiceTableModel()
  private val table = ServiceTable(tableModel)
  private val headerLabel = JBLabel("disconnected")
  private val countsLabel = JBLabel("")
  private val filterField = JBTextField()
  private val filterPanel = JPanel(BorderLayout())
  private var previousStatuses = emptyMap<String, ServiceStatus>()

  private var startAction: ServiceAction? = null
  private var stopAction: ServiceAction? = null
  private var restartAction: ServiceAction? = null
  private var runAction: ProcessAction? = null
  private var stopProcessAction: ProcessAction? = null
  private var profileSelector: ProfileComboBoxAction? = null

  init {
    setupTable()
    setupHeader()
    setupToolbar()
    setupContextMenu()
    setupKeybindings()
    setupFilter()

    val contentPanel = JPanel(BorderLayout())
    contentPanel.add(JBScrollPane(table), BorderLayout.CENTER)
    contentPanel.add(filterPanel, BorderLayout.SOUTH)
    setContent(contentPanel)

    PluginService.getInstance().addListener(this)
    processHandler.addListener(this)
  }

  private fun setupTable() {
    table.selectionModel.addListSelectionListener { updateActionState() }
  }

  private fun setupHeader() {
    val headerPanel = JPanel(BorderLayout())
    headerPanel.border =
      BorderFactory.createCompoundBorder(
        BorderFactory.createMatteBorder(0, 0, 1, 0, ServiceColors.muted),
        EmptyBorder(6, 10, 6, 10),
      )
    headerLabel.font = headerLabel.font.deriveFont(12f)
    headerLabel.foreground = ServiceColors.muted
    countsLabel.font = countsLabel.font.deriveFont(12f)
    headerPanel.add(headerLabel, BorderLayout.WEST)
    headerPanel.add(countsLabel, BorderLayout.EAST)
    add(headerPanel, BorderLayout.NORTH)
  }

  private fun setupToolbar() {
    startAction =
      ServiceAction("Start", "Start selected service", Icons.Start) { svc ->
        PluginService.getInstance().startService(svc.id)
      }
    stopAction =
      ServiceAction("Stop", "Stop selected service", Icons.Stop) { svc ->
        PluginService.getInstance().stopService(svc.id)
      }
    restartAction =
      ServiceAction("Restart", "Restart selected service", Icons.Restart) { svc ->
        PluginService.getInstance().restartService(svc.id)
      }

    profileSelector = ProfileComboBoxAction(processHandler)

    runAction =
      ProcessAction("Run", "Run fuku with selected profile", Icons.RunAll) {
        val profile = profileSelector?.selectedProfile ?: "default"
        processHandler.start(profile)
      }
    stopProcessAction =
      ProcessAction("Stop", "Stop fuku process", Icons.Stop) {
        processHandler.stop()
      }

    val settingsAction =
      object : AnAction("Settings", "Open fuku settings", Icons.Settings) {
        override fun actionPerformed(e: AnActionEvent) {
          ShowSettingsUtil.getInstance().showSettingsDialog(project, "Fuku")
        }

        override fun getActionUpdateThread() = ActionUpdateThread.EDT
      }

    val actionGroup =
      DefaultActionGroup().apply {
        add(startAction!!)
        add(stopAction!!)
        add(restartAction!!)
        addSeparator()
        add(profileSelector!!)
        add(runAction!!)
        add(stopProcessAction!!)
        addSeparator()
        add(settingsAction)
      }

    val toolbar =
      ActionManager.getInstance()
        .createActionToolbar(ActionPlaces.TOOLWINDOW_CONTENT, actionGroup, true)
    toolbar.targetComponent = this
    setToolbar(toolbar.component)

    updateActionState()
  }

  private fun setupKeybindings() {
    val inputMap = table.getInputMap(JComponent.WHEN_FOCUSED)
    val actionMap = table.actionMap

    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_J, 0), "moveDown")
    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_K, 0), "moveUp")
    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_S, 0), "toggleService")
    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_R, 0), "restartService")
    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_R, KeyEvent.CTRL_DOWN_MASK), "restartFailed")
    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_SLASH, 0), "showFilter")
    inputMap.put(KeyStroke.getKeyStroke(KeyEvent.VK_ESCAPE, 0), "clearFilter")

    actionMap.put(
      "moveDown",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) = moveSelectionDown()
      },
    )
    actionMap.put(
      "moveUp",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) = moveSelectionUp()
      },
    )
    actionMap.put(
      "toggleService",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) = toggleService()
      },
    )
    actionMap.put(
      "restartService",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) = restartSelectedService()
      },
    )
    actionMap.put(
      "restartFailed",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) = restartAllFailed()
      },
    )
    actionMap.put(
      "showFilter",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) = showFilter()
      },
    )
    actionMap.put(
      "clearFilter",
      object : AbstractAction() {
        override fun actionPerformed(e: ActionEvent) {
          if (tableModel.isFiltered) {
            filterField.text = ""
            tableModel.clearFilter()
            filterPanel.isVisible = false
          }
        }
      },
    )
  }

  private fun setupFilter() {
    filterPanel.isVisible = false
    filterPanel.border =
      BorderFactory.createCompoundBorder(
        BorderFactory.createMatteBorder(1, 0, 0, 0, ServiceColors.muted),
        EmptyBorder(4, 10, 4, 10),
      )

    val label = JBLabel("/ ")
    label.foreground = ServiceColors.muted
    filterPanel.add(label, BorderLayout.WEST)
    filterPanel.add(filterField, BorderLayout.CENTER)

    filterField.addKeyListener(
      object : KeyAdapter() {
        override fun keyPressed(e: KeyEvent) {
          when (e.keyCode) {
            KeyEvent.VK_ESCAPE -> {
              filterField.text = ""
              tableModel.clearFilter()
              filterPanel.isVisible = false
              table.requestFocusInWindow()
              e.consume()
            }
            KeyEvent.VK_ENTER -> {
              table.requestFocusInWindow()
              e.consume()
            }
          }
        }
      },
    )

    filterField.document.addDocumentListener(
      object : DocumentListener {
        override fun insertUpdate(e: DocumentEvent) = applyFilter()

        override fun removeUpdate(e: DocumentEvent) = applyFilter()

        override fun changedUpdate(e: DocumentEvent) = applyFilter()
      },
    )
  }

  private fun showFilter() {
    filterPanel.isVisible = true
    filterField.requestFocusInWindow()
  }

  private fun applyFilter() {
    tableModel.setFilter(filterField.text)
  }

  private fun moveSelectionDown() {
    val current = table.selectedRow
    if (current < 0) {
      for (i in 0 until table.rowCount) {
        if (!tableModel.isTierHeader(i)) {
          table.setRowSelectionInterval(i, i)
          table.scrollRectToVisible(table.getCellRect(i, 0, true))
          return
        }
      }
      return
    }
    var next = current + 1
    while (next < table.rowCount && tableModel.isTierHeader(next)) {
      next++
    }
    if (next < table.rowCount) {
      table.setRowSelectionInterval(next, next)
      table.scrollRectToVisible(table.getCellRect(next, 0, true))
    }
  }

  private fun moveSelectionUp() {
    val current = table.selectedRow
    if (current < 0) return
    var prev = current - 1
    while (prev >= 0 && tableModel.isTierHeader(prev)) {
      prev--
    }
    if (prev >= 0) {
      table.setRowSelectionInterval(prev, prev)
      table.scrollRectToVisible(table.getCellRect(prev, 0, true))
    }
  }

  private fun toggleService() {
    if (!isPhaseRunning()) return
    val service = getSelectedService() ?: return
    when {
      canStart(service) ->
        runServiceAction(service) {
          PluginService.getInstance().startService(it.id)
        }
      canStop(service) ->
        runServiceAction(service) {
          PluginService.getInstance().stopService(it.id)
        }
    }
  }

  private fun restartSelectedService() {
    if (!isPhaseRunning()) return
    val service = getSelectedService() ?: return
    if (!canRestart(service)) return
    runServiceAction(service) {
      PluginService.getInstance().restartService(it.id)
    }
  }

  private fun restartAllFailed() {
    if (!isPhaseRunning()) return
    val plugin = PluginService.getInstance()
    val failed =
      plugin.state.services
        .filter { it.status == ServiceStatus.failed }
    if (failed.isEmpty()) return

    ApplicationManager.getApplication().executeOnPooledThread {
      for (svc in failed) {
        try {
          plugin.restartService(svc.id).onFailure { ex ->
            log.warn("Failed to restart service ${svc.id}", ex)
          }
        } catch (ex: Exception) {
          log.warn("Failed to restart service ${svc.id}", ex)
        }
      }
      plugin.refresh()
    }
  }

  private fun runServiceAction(
    service: Service,
    action: (Service) -> Result<*>,
  ) {
    ApplicationManager.getApplication().executeOnPooledThread {
      try {
        val result = action(service)
        result.onSuccess { PluginService.getInstance().refresh() }
        result.onFailure { ex ->
          log.warn("Action failed for service ${service.id}", ex)
        }
      } catch (ex: Exception) {
        log.warn("Action failed for service ${service.id}", ex)
      }
    }
  }

  private fun setupContextMenu() {
    table.addMouseListener(
      object : MouseAdapter() {
        override fun mousePressed(e: MouseEvent) = handlePopup(e)

        override fun mouseReleased(e: MouseEvent) = handlePopup(e)

        private fun handlePopup(e: MouseEvent) {
          if (!e.isPopupTrigger) return
          val row = table.rowAtPoint(e.point)
          if (row < 0 || tableModel.isTierHeader(row)) return
          table.setRowSelectionInterval(row, row)

          val popupGroup =
            DefaultActionGroup().apply {
              add(startAction!!)
              add(stopAction!!)
              add(restartAction!!)
            }
          val popupMenu =
            ActionManager.getInstance()
              .createActionPopupMenu(ActionPlaces.POPUP, popupGroup)
          popupMenu.component.show(table, e.x, e.y)
        }
      },
    )
  }

  override fun onStateChanged(state: PluginState) {
    ApplicationManager.getApplication().invokeLater {
      val selectedId = getSelectedService()?.id
      tableModel.updateServices(state.services)

      if (selectedId != null) {
        val newRow = tableModel.findRowForServiceId(selectedId)
        if (newRow >= 0) {
          table.setRowSelectionInterval(newRow, newRow)
        }
      }

      if (state.connected && state.status != null) {
        val s = state.status
        headerLabel.text = "profile \u2022 ${s.profile}"
        headerLabel.foreground = ServiceColors.muted

        val c = s.services
        val phaseText =
          when (s.phase) {
            Phase.startup -> "starting"
            Phase.running -> "running"
            Phase.stopping -> "stopping"
            Phase.stopped -> "stopped"
          }
        countsLabel.text = "$phaseText ${c.running}/${c.total} ready"
        countsLabel.foreground = ServiceColors.forPhase(s.phase)

        checkForFailures(state.services)
      } else {
        headerLabel.text = "disconnected"
        headerLabel.foreground = ServiceColors.muted
        countsLabel.text = ""
      }

      updateActionState()
    }
  }

  private fun checkForFailures(services: List<Service>) {
    for (svc in services) {
      if (svc.status != ServiceStatus.failed) continue
      val prev = previousStatuses[svc.id] ?: continue
      if (prev == ServiceStatus.failed) continue
      showFailureNotification(svc)
    }
    previousStatuses = services.associate { it.id to it.status }
  }

  private fun showFailureNotification(service: Service) {
    val notification =
      NotificationGroupManager.getInstance()
        .getNotificationGroup("fuku")
        .createNotification(
          "${service.name} failed",
          service.error ?: "Unknown error",
          NotificationType.WARNING,
        )

    notification.addAction(
      object : AnAction("Restart") {
        override fun actionPerformed(e: AnActionEvent) {
          if (!isPhaseRunning()) return
          ApplicationManager.getApplication().executeOnPooledThread {
            PluginService.getInstance().restartService(service.id)
            PluginService.getInstance().refresh()
          }
          notification.expire()
        }
      },
    )
    notification.notify(project)
  }

  override fun onProcessStateChanged(running: Boolean) {
    ApplicationManager.getApplication().invokeLater {
      updateActionState()
    }
  }

  private fun getSelectedService(): Service? {
    val row = table.selectedRow
    if (row < 0) return null
    return tableModel.getServiceAt(row)
  }

  private fun isPhaseRunning(): Boolean = PluginService.getInstance().state.status?.phase == Phase.running

  private fun updateActionState() {
    val service = getSelectedService()
    val phaseOk = isPhaseRunning()
    startAction?.isEnabled = phaseOk && service != null && canStart(service)
    stopAction?.isEnabled = phaseOk && service != null && canStop(service)
    restartAction?.isEnabled = phaseOk && service != null && canRestart(service)
    runAction?.isEnabled = !processHandler.isRunning
    stopProcessAction?.isEnabled = processHandler.isRunning
  }

  private fun canStart(service: Service): Boolean = service.status == ServiceStatus.stopped || service.status == ServiceStatus.failed

  private fun canStop(service: Service): Boolean = service.status == ServiceStatus.running

  private fun canRestart(service: Service): Boolean =
    service.status == ServiceStatus.running ||
      service.status == ServiceStatus.stopped ||
      service.status == ServiceStatus.failed

  fun dispose() {
    table.dispose()
    PluginService.getInstance().removeListener(this)
    processHandler.removeListener(this)
  }

  private inner class ServiceAction(
    text: String,
    description: String,
    icon: Icon?,
    private val action: (Service) -> Result<*>,
  ) : AnAction(text, description, icon) {
    var isEnabled: Boolean = false

    override fun actionPerformed(e: AnActionEvent) {
      val service = getSelectedService() ?: return
      ApplicationManager.getApplication().executeOnPooledThread {
        try {
          val result = action(service)
          result.onSuccess { PluginService.getInstance().refresh() }
          result.onFailure { ex ->
            log.warn("Action failed for service ${service.id}", ex)
            ApplicationManager.getApplication().invokeLater {
              Messages.showErrorDialog(
                project,
                "Action failed for ${service.name}: ${ex.message}",
                "fuku",
              )
            }
          }
        } catch (ex: Exception) {
          log.warn("Action failed for service ${service.id}", ex)
        }
      }
    }

    override fun update(e: AnActionEvent) {
      e.presentation.isEnabled = isEnabled
    }

    override fun getActionUpdateThread() = ActionUpdateThread.EDT
  }

  private inner class ProcessAction(
    text: String,
    description: String,
    icon: Icon?,
    private val action: () -> Unit,
  ) : AnAction(text, description, icon) {
    var isEnabled: Boolean = false

    override fun actionPerformed(e: AnActionEvent) {
      action()
    }

    override fun update(e: AnActionEvent) {
      e.presentation.isEnabled = isEnabled
    }

    override fun getActionUpdateThread() = ActionUpdateThread.EDT
  }

  private class ProfileComboBoxAction(
    private val processHandler: ProcessHandler,
  ) : ComboBoxAction() {
    @Volatile
    var selectedProfile: String = "default"
      private set

    override fun createPopupActionGroup(button: JComponent): DefaultActionGroup {
      val profiles = processHandler.readProfiles()
      if (selectedProfile !in profiles && profiles.isNotEmpty()) {
        selectedProfile = profiles[0]
      }

      val group = DefaultActionGroup()
      for (profile in profiles) {
        group.add(
          object : AnAction(profile) {
            override fun actionPerformed(e: AnActionEvent) {
              selectedProfile = profile
            }

            override fun getActionUpdateThread() = ActionUpdateThread.EDT
          },
        )
      }
      return group
    }

    override fun update(e: AnActionEvent) {
      e.presentation.text = selectedProfile
      e.presentation.isEnabled = !processHandler.isRunning
    }

    override fun getActionUpdateThread() = ActionUpdateThread.EDT
  }
}
