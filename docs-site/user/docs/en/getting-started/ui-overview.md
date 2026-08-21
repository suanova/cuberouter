# Interface Overview

> Welcome to CubeRouter! This guide gives you a quick tour of the platform interface.

## The Platform Interface at a Glance

CubeRouter uses a modern responsive design with a clean, intuitive UI. After signing in, the console is made up of a **left sidebar** and a **main content area**; public pages (Home, Model Square, etc.) use a **top navigation bar** and the main content area.

![Platform home page](/imgs-en/home.jpeg)

## Top Navigation Bar

![Console interface](/imgs-en/dashboard.jpeg)

The top navigation bar contains the logo, navigation items, and a right-side action area. Navigation items are determined by the site configuration and usually include by default:

- **Logo**: click to go to the platform home page, which shows product highlights, usage steps, and API call examples
- **Model Square**: browse the list of available models and pricing
- **Rankings**: view model usage rankings
- **About**: learn more about the platform

## Top-Right Area

- **Notifications**: when there are unread system announcements or in-app notifications, a red numeric badge appears above the icon; click it to view system announcements and account notifications
- **Language switcher**: supports Simplified Chinese, Traditional Chinese, English, and more; the choice is saved automatically
- **Theme switcher**: light mode, dark mode, and automatic (follow system), plus color options
- **User area**:
  - Signed out: shows the **Sign In** button
  - Signed in: shows your avatar (initial) and username; click to open a dropdown menu:
    - **Profile**: edit personal info, security settings, and more
    - **Wallet**: check your balance and top up
    - **Sign out**: sign out of the current account

## Left Sidebar (Console)

After signing in and entering the console, the left sidebar provides the following entry points:

| Menu | Description |
| --- | --- |
| Playground | Test model conversations online |
| Overview | Account usage summary, API info, announcements, and FAQs |
| Dashboard | Multi-dimensional statistics on call volume, token consumption, and performance |
| API Keys | Create and manage keys for API calls |
| Usage Logs | View detailed records of every API call |
| Task Logs | View async tasks such as image drawing and music generation |
| Wallet | Balance management, top-up, redemption codes, and referral rewards |
| Profile | Personal settings and more |

> Users with different roles see different menus: admin and operations users also see extra menus such as channel management, model management, user management, and system settings.

## Overview Page

The default page after signing in is **Overview**, which shows:

![Overview page](/imgs-en/dashboard.jpeg)

- **Setup guide**: a three-step guide — create an API key, add quota, send a request
- **Usage at a glance**: last 24h usage, historical usage, request count, credit remaining
- **API Info**: API address and documentation entry point
- **Announcements**: the latest system announcements
- **FAQ**: common questions you can expand to view
- **Uptime**: service performance metrics

🎉 **Congratulations! You now know the basic layout of the CubeRouter interface.** Next, follow the [Quick Start](./quick-start.md) to make your first API call.
