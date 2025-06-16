# 📨 RSS Mailer

A Lambda function for turning your RSS feed items into emails.

- [📨 RSS Mailer](#-rss-mailer)
  - [About the Project](#about-the-project)
  - [Getting Started](#getting-started)
    - [Environment Values](#environment-values)
    - [Expected Arguments](#expected-arguments)
  - [License](#license)
  - [Contributing](#contributing)
    - [Open an Issue](#open-an-issue)
    - [Send a Pull Request](#send-a-pull-request)

## About the Project

This daughter project of SimpleSubscribe contains code that you upload to an AWS Lambda function. The Lambda receives a trigger event from your RSS feed, and uses this data to send emails to your subscriber list stored in Dynamo DB.

For building your own independent subscriber base, see the [SimpleSubscribe project](https://github.com/victoriadrake/simple-subscribe).

## Getting Started

1. [Create a Lambda function](https://docs.aws.amazon.com/lambda/latest/dg/getting-started-create-function.html) to house RSS Mailer.
2. Create a `.env` file. See below for values.
3. Upload the function code. If your env is set up, you can run `make update`.
4. Set up your trigger to provide the expected arguments. See below.

### Environment Values

Your `.env` is used by the helper scripts and provides an easy way to upload Lambda environment variables. Here's an example of the expected values that you can customize for your needs:

```text
FUNCTION_NAME="rss-mailer"

LAMBDA_ENV="Variables={\
DB_TABLE_NAME=SimpleSubscribeList,\
UNSUBSCRIBE_LINK=https://example.com/api/unsubscribe/,\
WEBSITE=https://example.com,\
TITLE='My Newsletter Title',\
SENDER_EMAIL=no-reply@example.com,\
SENDER_NAME='Arthur Dent'}"
```

### Expected Arguments

The function's `Invocation` struct shows the arguments expected to be passed in from the trigger event. These are:

- `Title`: The title of an RSS item.
- `Description`: A description of an RSS item.
- `Content`: The main content of the item, usually HTML formatted.
- `Plain`: A plain-text version of the content.
- `Link`: A link to the item on the web.

If, for example, you're using Zapier to trigger your function, ensure that each of these arguments are provided in the **Invoke Function in AWS Lambda** > **Set up action** step.

## License

This project is available under the [Mozilla Public License 2.0 (MPL-2.0)](https://www.mozilla.org/en-US/MPL/2.0/).

## Contributing

RSS Mailer would be happy to have your contribution! Add helper scripts, improve the code, or even just fix a typo you found.

Here are a couple ways you can help out. Thank you for being a part of this open source project! 💕

### Open an Issue

Please open an issue to report bugs or anything that might need fixing or updating.

### Send a Pull Request

If you would like to change or fix something yourself, a pull request (PR) is most welcome! Please open an issue before you start working. That way, you can let other people know that you're taking care of it and no one ends up doing extra work.

Please [fork the repository](https://help.github.com/en/github/getting-started-with-github/fork-a-repo), then check out a local branch that includes the issue number, such as `fix-<issue number>`. For example, `git checkout -b fix-42`.

Before you submit your PR, make sure your fork is [synced with `master`](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/syncing-a-fork), then [create a PR](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/creating-a-pull-request-from-a-fork). You may want to [allow edits from maintainers](https://help.github.com/en/github/collaborating-with-issues-and-pull-requests/allowing-changes-to-a-pull-request-branch-created-from-a-fork) so that I can help with small changes like fixing typos.
