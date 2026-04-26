package com.fuku.plugin.run

import com.fuku.plugin.Icons
import com.fuku.plugin.settings.Settings
import com.intellij.execution.Executor
import com.intellij.execution.configurations.GeneralCommandLine
import com.intellij.execution.configurations.RunConfigurationBase
import com.intellij.execution.configurations.RunConfigurationOptions
import com.intellij.execution.configurations.SimpleConfigurationType
import com.intellij.execution.process.KillableColoredProcessHandler
import com.intellij.execution.process.ProcessHandler
import com.intellij.execution.runners.ExecutionEnvironment
import com.intellij.openapi.options.SettingsEditor
import com.intellij.openapi.project.Project
import com.intellij.openapi.util.NotNullLazyValue
import com.intellij.ui.components.JBLabel
import com.intellij.util.ui.FormBuilder
import org.jdom.Element
import java.io.File
import javax.swing.DefaultComboBoxModel
import javax.swing.JComboBox
import javax.swing.JComponent

class ConfigurationType : SimpleConfigurationType(
  "FukuRunConfiguration",
  "fuku",
  "Run fuku with a profile",
  NotNullLazyValue.createValue { Icons.ToolWindow },
) {
  override fun createTemplateConfiguration(project: Project) = RunConfiguration(project, this, "fuku")
}

class RunConfiguration(
  project: Project,
  type: SimpleConfigurationType,
  name: String,
) : RunConfigurationBase<RunConfigurationOptions>(project, type, name) {
  var profile: String = "default"

  override fun getState(
    executor: Executor,
    environment: ExecutionEnvironment,
  ) = CommandLineState(environment, this)

  override fun getConfigurationEditor() = RunConfigurationEditor(project)

  override fun readExternal(element: Element) {
    super.readExternal(element)
    profile = element.getAttributeValue("profile") ?: "default"
  }

  override fun writeExternal(element: Element) {
    super.writeExternal(element)
    element.setAttribute("profile", profile)
  }
}

class RunConfigurationEditor(
  private val project: Project,
) : SettingsEditor<RunConfiguration>() {
  private val profileComboBox = JComboBox<String>()

  override fun createEditor(): JComponent {
    loadProfiles()
    return FormBuilder.createFormBuilder()
      .addLabeledComponent(JBLabel("Profile:"), profileComboBox)
      .panel
  }

  override fun applyEditorTo(s: RunConfiguration) {
    s.profile = profileComboBox.selectedItem as? String ?: "default"
  }

  override fun resetEditorFrom(s: RunConfiguration) {
    loadProfiles()
    profileComboBox.selectedItem = s.profile
  }

  private fun loadProfiles() {
    val handler = project.getService(com.fuku.plugin.run.ProcessHandler::class.java)
    val profiles = handler.readProfiles()
    profileComboBox.model = DefaultComboBoxModel(profiles.toTypedArray())
  }
}

class CommandLineState(
  environment: ExecutionEnvironment,
  private val configuration: RunConfiguration,
) : com.intellij.execution.configurations.CommandLineState(environment) {
  override fun startProcess(): ProcessHandler {
    val settings = Settings.getInstance()
    val commandLine =
      GeneralCommandLine(
        settings.fukuBinaryPath,
        "--no-ui",
        "run",
        configuration.profile,
      )
    commandLine.workDirectory = environment.project.basePath?.let { File(it) }
    commandLine.charset = Charsets.UTF_8
    return KillableColoredProcessHandler(commandLine)
  }
}
