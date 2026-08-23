# Vcode Plugin Packages

Vcode plugin packages bundle skills, hooks, and MCP servers behind one
installable unit.

## CLI Mode

Use `Vcode plugin` when installing or managing plugin packages from a
terminal. Plugin packages are installed globally under the Vcode home
directory.

### Install From CLI

`install` accepts one source:

- A GitHub repository, such as `git:github.com/obra/superpowers` or
  `https://github.com/obra/superpowers`.
- A GitHub branch or subdirectory URL, such as
  `https://github.com/owner/repo/tree/main/path/to/plugin`.
- A local directory that contains `Vcode-plugin.json` or
  `.codex-plugin/plugin.json`.

Preview the install plan without writing files:

```bash
Vcode plugin install git:github.com/obra/superpowers --dry-run
```

Install a plugin after reviewing the plan:

```bash
Vcode plugin install git:github.com/obra/superpowers --yes
```

Install with an explicit name or replace an installed plugin with the same name:

```bash
Vcode plugin install git:github.com/obra/superpowers --name superpowers --replace --yes
```

Use a local directory in developer mode:

```bash
Vcode plugin install /path/to/plugin --link --replace --yes
```

CLI install flags:

- `--dry-run` plans and validates the install without writing files.
- `--yes` is required for any install that writes files.
- `--replace` allows the source to replace an installed plugin with the same
  name.
- `--name <name>` or `--name=<name>` overrides the name from the plugin
  manifest for this install.
- `--link` links a local plugin directory instead of copying it into Vcode's
  plugin storage. Moving or deleting that directory breaks the linked plugin.

Running `Vcode plugin install <source>` without `--dry-run` or `--yes`
refuses to write files and prints a reminder to rerun with one of those flags.
Install and remove commands print the structured JSON response from the same
install-source backend used by the web UI.

Installed plugin state is stored in:

```text
~/.Vcode/plugin-packages.json
~/.Vcode/plugins/<name>/
```

### Manage From CLI

List installed plugins:

```bash
Vcode plugin list
```

Show one plugin's metadata, root, source, and exported capability counts:

```bash
Vcode plugin show superpowers
```

`show` also prints the concrete capability inventory when available:

- **skills** include suggested `/<skill>` invocations and descriptions.
- **hooks** list lifecycle events, matchers, and commands or context files.
- **mcpServers** list server names, transports, and launch targets.

Check that the manifest and skill roots are readable:

```bash
Vcode plugin doctor superpowers
```

Enable or disable a plugin without uninstalling it:

```bash
Vcode plugin disable superpowers
Vcode plugin enable superpowers
```

Remove a plugin:

```bash
Vcode plugin remove superpowers --yes
```

`remove` also accepts `uninstall` as an alias. It requires `--yes` because it
writes state and removes copied plugin content. For linked local plugins, the
external source directory is left in place.

### Use Installed Plugins From CLI

Installed plugins do not open a separate chat surface. When a plugin is enabled,
Vcode loads its capabilities into normal interactive sessions:

- Run `/plugins` inside an interactive session to list installed plugin
  packages. Run `/plugins show <name>` to inspect a plugin's exported skills,
  hooks, MCP servers, and usage hints without leaving the chat.
- **Skills** appear in `/skills`. Invoke a skill with `/<skill> [args]`, or ask
  naturally and let the agent choose a matching skill by description.
- **Hooks** run automatically at their configured lifecycle events, such as
  `SessionStart`, `UserPromptSubmit`, `PreToolUse`, or `PostToolUse`.
- **MCP servers** join the normal MCP/tool flow. Ask for the task you want done;
  Vcode can call the plugin's tools when they are relevant.

After installing, enabling, disabling, or updating a plugin from a separate
terminal while a session is already running, start a new `Vcode` session or
reopen `/skills` to verify the current session sees the expected skills.

## Web UI Settings

Open **Settings -> Plugins** to install and manage plugin packages without using
the CLI.

### Install Plugins

The installer has two modes:

- **Local folder**: click **Choose plugin folder** and select a plugin directory
  on disk. The selected path is shown next to the button.
- **Git repository**: enter a Git source such as
  `git:github.com/obra/superpowers`. **Install name (optional)** can override
  the plugin manifest name for this install or overwrite.

Use the action buttons after choosing the source and options:

- **Preview** validates the source and shows the planned install actions without
  writing files.
- **Install plugin** installs the selected source using the current options.
- **Refresh plugins** reloads the installed-plugin list from disk and config.

Installer options:

- **Overwrite same-name plugin** allows the current source to replace an
  installed plugin with the same name. Leave it off when duplicate-name installs
  should fail instead of replacing existing content.
- **Developer mode: link source folder** appears for **Local folder** installs.
  It links the selected directory instead of copying it into Vcode's plugin
  storage. Use it while developing or debugging a plugin. Moving or deleting the
  selected directory will break the linked plugin.

Preview is the safest first step for a new Git source or local plugin directory.

### Manage Installed Plugins

The installed-plugin list shows each plugin package and its exported skills,
hooks, and MCP servers. Use **Refresh plugins** after editing plugin files or
changing config outside the app.

Expand a plugin row to manage it:

- Enable or disable the plugin.
- Read **How to use** for the plugin's exported skills, hooks, and MCP servers.
- **Update** pulls or refreshes an installed plugin when an update source is
  available.
- **Doctor** checks the plugin manifest and reports warnings or diagnostics.
- **Remove plugin** uninstalls the package after confirmation.

### Use Installed Plugins From the Web UI

The web UI uses the same runtime model as the CLI:

- Expand an installed plugin to see its **How to use** section.
- In any session, type `/plugins` to list installed plugins, or
  `/plugins show <name>` to see the same usage details from the chat surface.
- Skills are shown with suggested direct commands such as `/plan`; they are also
  discoverable from `/skills` in a session.
- Hooks and MCP servers are listed for transparency. They do not need a manual
  "run" button: enabled hooks trigger automatically, and MCP tools are available
  through ordinary tool use.
- If a currently open session does not reflect a plugin change, refresh the
  plugin list and open a new session.

## Native Manifest

Vcode plugins can declare `Vcode-plugin.json` at the plugin root:

```json
{
  "name": "example",
  "version": "1.0.0",
  "description": "Example plugin",
  "skills": "skills",
  "hooks": {
    "SessionStart": [
      {
        "command": "hooks/session-start",
        "description": "Load startup context"
      }
    ]
  },
  "mcpServers": {
    "helper": {
      "command": "bin/helper"
    }
  }
}
```

Relative paths are resolved inside the plugin root. Vcode does not run
third-party install scripts during plugin installation.

## Codex Compatibility

Vcode also reads Codex plugin manifests at `.codex-plugin/plugin.json`.
For packages such as Superpowers and Claude-style skill packs, Vcode maps:

- `skills` to Vcode skill roots.
- `hooks/session-start-codex` to the Vcode `SessionStart` hook when present.
- A plugin-root `CLAUDE.md` file to a built-in `SessionStart` context hook. The
  file is read directly by Vcode, without spawning a shell command.
- `.claude/settings.json` command hooks to Vcode hook events when the event
  names match. Claude's `matcher` field maps to Vcode `match`; hook commands
  run as shell commands with the plugin root as `cwd`; Claude `timeout` values
  are interpreted as seconds.

Unsupported Claude hook item types are skipped with a warning. Vcode does not
run third-party install scripts or implement marketplace-specific install
protocols.

Plugin hooks receive these environment variables:

- `Vcode_PLUGIN_ROOT`
- `Vcode_PLUGIN_NAME`
- `Vcode_PLUGIN_VERSION`
- `Vcode_HOME`
- `Vcode_WORKSPACE_ROOT`
- `CLAUDE_PROJECT_DIR`
