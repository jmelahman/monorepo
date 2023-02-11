#!/usr/bin/env python3
import io
import os

import tweepy  # type: ignore[import]


def write_tweets(tweets_file: io.TextIOWrapper, tweets: tweepy.Tweet) -> None:  # type: ignore[no-any-unimported]
    for tweet in tweets:
        if not tweet.text or tweet.text.startswith("@") or tweet.text.startswith("RT"):
            continue
        tweets_file.write(tweet.text + "\n")


def main() -> None:
    client = tweepy.Client(os.environ["ACCESS_TOKEN"])
    user = client.get_user(username="jmelahman")
    tweets = client.get_users_tweets(user.data.id)
    with open("my_tweets.txt", "w+") as my_tweets:
        write_tweets(my_tweets, tweets)
        while tweets.meta.get("next_token"):
            tweets = client.get_users_tweets(
                user.data.id, pagination_token=tweets.meta["next_token"]
            )
            write_tweets(my_tweets, tweets.data)


if __name__ == "__main__":
    main()
