# Mattermost Inactive User Cleaner

## Overview

This utility enables Mattermost administrators to efficiently identify and deactivate inactive users within a specified Mattermost team. By deactivating inactive users, administrators can effectively manage and free up licensed seats, ensuring that only active users are counted against the Mattermost license. This tool is particularly useful for large teams looking to optimise their Mattermost license usage.

## Features

- **Identify Inactive Users**: Automatically find users who have been inactive for a specified number of days (default is 180 days).
- **Deactivate Users**: Option to deactivate identified inactive users, freeing up licensed seats.
- **Dry-Run Mode**: Assess which users would be deactivated without making any actual changes, allowing for review and decision-making.
- **Debug Mode**: Provides additional output for troubleshooting and verification purposes.

## Prerequisites

- A personal access token from your Mattermost instance, with appropriate permissions. [More Information](https://developers.mattermost.com/integrate/reference/personal-access-token/)
- Knowledge of your Mattermost URL, the HTTP scheme (http/https), and the port in use.

## Installation

Download the latest release from the GitHub releases page. No Go development environment is needed to run the utility, as executables are provided for various platforms. Unzip the package in a directory of your choice.

## Usage

Here's how to use the Mattermost Inactive User Cleaner:

```bash
./mm-inactive-cleaner -url your_mattermost_url -port your_port -scheme http/https -token your_personal_access_token -team team_name -age days_of_inactivity -dry-run
```

### Parameters

| Command Line Arg | Environment Variable | Description |
| --- | --- | --- |
| `-url` | `MM_URL` | The URL of the Mattermost instance (without a schema) |
| `-port` | `MM_PORT` | The Mattermost port to be used [default: 8065] |
| `-scheme` | `MM_SCHEME` | The HHTP scheme to be used (http/https) [default: http] |
| `-token` | `MM_TOKEN` | The user token for Mattermost.  Note that this user must have the appropriate rights to read users. |
| `-team` |  | The Mattermost Team name that the cleanup should be applied to |
| `-age` |  | Age (in days) for an inactive user to be deactivated. [default: 180] |
| `-dry-run` |  | If present, the list of users to be deactivated will be displayed on the screen, but no action will be taken. |
| `-debug` | `MM_DEBUG` | If present, will run in debug mode, delivering additional output to stdout |

## Contributing

We welcome contributions from the community! Whether it's a bug report, a feature suggestion, or a pull request, your input is valuable to us. Please feel free to contribute in the following ways:

- **Issues and Pull Requests**: For specific questions, issues, or suggestions for improvements, open an issue or a pull request in this repository.
- **Mattermost Community**: Join the discussion in the [Integrations and Apps](https://community.mattermost.com/core/channels/integrations) channel on the Mattermost Community server.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contact

For questions, feedback, or contributions regarding this project, please use the following methods:

- **Issues and Pull Requests**: For specific questions, issues, or suggestions for improvements, feel free to open an issue or a pull request in this repository.
- **Mattermost Community**: Join us in the Mattermost Community server, where we discuss all things related to extending Mattermost. You can find me in the channel [Integrations and Apps](https://community.mattermost.com/core/channels/integrations).
- **Social Media**: Follow and message me on Twitter, where I'm [@jlandells](https://twitter.com/jlandells).