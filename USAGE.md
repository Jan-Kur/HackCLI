**I know it seems like a lot of work, but it really isn't, takes like a minute and it's really worth it, don't be the average andy that can't survive 60 seconds without dopamine**

## Get the slack cookie (login with slack)
First of all, you need to get a slack cookie from your slack browser session.

<details>
<summary>Is this safe?</summary>

**yes** - I will be transparent here: normal slack api endpoints aren't sufficient for HackCLI's use case, so instead we ask you to get the cookie (from which the config wizard then gets the xoxc token) so that HackCLI can have the same privilages as your normal slack browser session and give you the slack experience. This approach is used by other projects too.
Don't worry about your important data being stolen or whatever. HackCLI can only access what you can already see in Slack. On top of that, I don't steal any data and have no interest in doing so.
</details>

The experience might vary in different browsers, but is similar in general. Here are the steps:

1. Visit https://hackclub.slack.com/ and log in if necessary. Make sure to choose the Hack Club workspace, HackCLI works only with it.
2. Once you are inside the Hack Club workspace, open developer tools and head to the storage section (in Chrome this is under Application -> Storage). Under Cookies expand `https://app.slack.com`. You will see a bunch of cookies but what we care about is under the name "d". Click on it's value and copy it (it should start with **xoxd**).
3. Make sure to have this copied cookie ready, we will need it in the next section.

## Configure your HackCLI app
1. You should have the app installed by now. To configure your app run:
      ```bash
      hackcli init
      ```
2. Then just follow the instructions of our config wizard: paste the slack cookie and choose whichever color theme you prefer. It will be used across your HackCLI app.

Btw you can always change these settings in (your config dir): `/home/username/.config/HackCLI/config.json` on Linux, `~/Library/Application Support/` on MacOS and `C:\Users\username\AppData\Roaming` on Windows.

## Usage - keybinds, functionality
I tried to mimic the slack UX, so using HackCLI should be straightforward, but with HackCLI you use your keyboard instead of a mouse (like every sane programmer, get over it!), so it's useful to know the keybinds instead of guessing. Here's a rough guide:

#### General:
- *tab* and *shift+tab* to switch between sidebar, chat, input etc
- ↑ and ↓ select next or previous item. It's indicated by a bright color border.

#### Sidebar:
- *enter* to open the selected channel/dm
- *j* to join a channel (you have to write it's ID, found in channel details in Slack)
- *l* to leave the selected channel

#### Chat: 
- *j* and *k* to select an item without moving the chat (↑ and ↓ move the chat to keep the message visible)
- *enter* to open a thread in a new window like in Slack (if the message has replies)
- *r* to add a reaction or remove it if you have already reacted with it
- *d* to delete a message if you sent it
- *e* to edit a message if you sent it

#### Input:
- *enter* to add a new line
- *alt+enter* to send the message

## Run HackCLI - FINALLY!
```bash
hackcli announcements # <- provide the channel or DM username you want to open first, by default opens the first channel alphabetically.
```







