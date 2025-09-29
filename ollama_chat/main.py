# /// script
# requires-python = ">=3.13"
# dependencies = [
#     "ollama",
#     "textual",
# ]
# ///
from __future__ import annotations

import dataclasses
from datetime import datetime
import enum
import json
import os
import sqlite3
import typing
import uuid

import ollama
from textual.app import App
from textual.app import ComposeResult
from textual.binding import Binding
from textual.containers import Container
from textual.containers import ScrollableContainer
from textual.containers import Vertical
from textual.screen import ModalScreen
from textual.widgets import Button
from textual.widgets import Footer
from textual.widgets import Input
from textual.widgets import Label
from textual.widgets import Markdown
from textual.widgets import RichLog

DEBUG = False

# Settings file path
SETTINGS_PATH = os.path.join(
    os.getenv("XDG_CONFIG_HOME", os.path.join(os.path.expanduser("~"), ".config")),
    "ollama-chat",
    "settings.json",
)

# Database setup
DB_PATH = os.path.join(
    os.getenv("XDG_STATE_HOME", os.path.join(os.path.expanduser("~"), ".local", "state")),
    "ollama-chat",
    "conversations.db",
)

# Default settings
DEFAULT_SETTINGS = {
    "OLLAMA_HOST": "http://localhost:11434",
    "MAIN_MODEL": "qwen3:8b",
    "WEAK_MODEL": "qwen3:0.6b",
}


class Role(enum.Enum):
    USER = "user"
    ASSISTANT = "assistant"


class Message(Markdown):
    def __init__(self, content: str, role: Role) -> None:
        super().__init__(content)
        self.role = role
        self.content = content
        self.add_class("message")
        self.add_class(role.value)


@dataclasses.dataclass
class Conversation:
    summary: str
    id: str
    messages: list[Message]
    created_at: str

    def __init__(self, summary: str = "", messages: list[Message] = []) -> None:
        self.id = "id-" + str(  # Textual buttons require the #id to start with a letter.
            uuid.uuid4()
        )
        self.summary = summary
        self.messages = messages
        self.created_at = datetime.now().isoformat()


def load_settings() -> dict:
    """Load settings from file or return defaults."""
    if os.path.exists(SETTINGS_PATH):
        try:
            with open(SETTINGS_PATH) as f:
                settings = json.load(f)
                # Merge with defaults to ensure all keys exist
                return {**DEFAULT_SETTINGS, **settings}
        except (json.JSONDecodeError, KeyError):
            pass
    return DEFAULT_SETTINGS.copy()


def save_settings(settings: dict) -> None:
    """Save settings to file."""
    os.makedirs(os.path.dirname(SETTINGS_PATH), exist_ok=True)
    with open(SETTINGS_PATH, "w") as f:
        json.dump(settings, f, indent=2)


# Load initial settings
SETTINGS = load_settings()
OLLAMA_HOST = SETTINGS["OLLAMA_HOST"]


def init_db() -> None:
    # Ensure directory exists
    os.makedirs(os.path.dirname(DB_PATH), exist_ok=True)

    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("""
        CREATE TABLE IF NOT EXISTS conversations (
            id TEXT PRIMARY KEY,
            summary TEXT,
            messages TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    """)
    conn.commit()
    conn.close()


def save_conversation(conversation: Conversation) -> None:
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute(
        """
        INSERT OR REPLACE INTO conversations (id, summary, messages, created_at)
        VALUES (?, ?, ?, ?)
    """,
        (
            conversation.id,
            conversation.summary,
            json.dumps(
                [{"content": msg.content, "role": msg.role.value} for msg in conversation.messages]
            ),
            conversation.created_at,
        ),
    )
    conn.commit()
    conn.close()


def load_conversation_summaries(search_term: str = "") -> list[tuple[str, str, str]]:
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()

    if search_term:
        # Full-text search across summary and messages
        query = """
            SELECT DISTINCT c.id, c.summary, c.created_at
            FROM conversations c
            LEFT JOIN (
                SELECT c.id as msg_id, json_extract(value, '$.content') as content
                FROM conversations c
                JOIN json_each(c.messages) je
            ) m ON c.id = m.msg_id
            WHERE c.summary LIKE ? OR m.content LIKE ?
            ORDER BY c.created_at DESC;
        """
        search_pattern = f"%{search_term}%"
        cursor.execute(query, (search_pattern, search_pattern))
    else:
        cursor.execute(
            "SELECT id, summary, created_at FROM conversations ORDER BY created_at DESC;"
        )

    rows = cursor.fetchall()
    conn.close()
    return rows


def load_conversation_by_id(convo_id: str) -> Conversation | None:
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute(
        "SELECT id, summary, messages, created_at FROM conversations WHERE id = ?", (convo_id,)
    )
    row = cursor.fetchone()
    conn.close()

    if row:
        convo_id, summary, messages_json, created_at = row
        messages_data = json.loads(messages_json) if messages_json else []
        messages = [Message(msg["content"], Role(msg["role"])) for msg in messages_data]
        convo = Conversation(summary, messages)
        convo.id = convo_id
        convo.created_at = created_at
        return convo
    return None


def delete_conversation(convo_id: str) -> None:
    conn = sqlite3.connect(DB_PATH)
    cursor = conn.cursor()
    cursor.execute("DELETE FROM conversations WHERE id = ?", (convo_id,))
    conn.commit()
    conn.close()


class SettingsScreen(ModalScreen):
    """Modal screen for settings."""

    BINDINGS = [
        Binding("escape", "app.pop_screen", "Close"),
    ]

    def __init__(self, settings: dict) -> None:
        super().__init__()
        self.settings = settings.copy()

    def compose(self) -> ComposeResult:
        with Container(id="settings-dialog"):
            yield Label("Settings", id="settings-title")

            yield Label("Ollama Host:")
            yield Input(
                value=self.settings["OLLAMA_HOST"],
                id="ollama-host-input",
                placeholder="http://localhost:11434",
            )

            yield Label("Main Model:")
            yield Input(
                value=self.settings["MAIN_MODEL"], id="main-model-input", placeholder="qwen3:0.6b"
            )

            yield Label("Weak Model:")
            yield Input(
                value=self.settings["WEAK_MODEL"], id="weak-model-input", placeholder="qwen3:0.6b"
            )

            with Container(id="settings-buttons"):
                yield Button("Save", variant="primary", id="save-settings")
                yield Button("Cancel", id="cancel-settings")

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "save-settings":
            # Update settings from input values
            self.settings["OLLAMA_HOST"] = self.query_one("#ollama-host-input").value
            self.settings["MAIN_MODEL"] = self.query_one("#main-model-input").value
            self.settings["WEAK_MODEL"] = self.query_one("#weak-model-input").value

            # Save to file
            save_settings(self.settings)

            # Notify parent app to reload settings
            self.app.reload_settings(self.settings)

            # Close modal
            self.app.pop_screen()
        elif event.button.id == "cancel-settings":
            # Close modal without saving
            self.app.pop_screen()


class DeleteConfirmationScreen(ModalScreen):
    """Modal screen for delete confirmation."""

    BINDINGS = [
        Binding("escape", "app.pop_screen", "Cancel"),
    ]

    def __init__(self, conversation_id: str, conversation_summary: str) -> None:
        super().__init__()
        self.conversation_id = conversation_id
        self.conversation_summary = conversation_summary

    def compose(self) -> ComposeResult:
        with Container(id="delete-dialog"):
            yield Label("Delete Conversation", id="delete-title")
            yield Label(
                f"Are you sure you want to delete '{self.conversation_summary}'?",
                id="delete-prompt",
            )

            with Container(id="delete-buttons"):
                yield Button("Delete", variant="error", id="confirm-delete")
                yield Button("Cancel", id="cancel-delete")

    def on_button_pressed(self, event: Button.Pressed) -> None:
        if event.button.id == "confirm-delete":
            # Delete the conversation
            delete_conversation(self.conversation_id)

            # Refresh the conversation list
            self.app.refresh_conversation_list()

            # If this was the current conversation, clear the chat
            if self.app.conversation and self.app.conversation.id == self.conversation_id:
                self.app.clear_current_conversation()

            # Close modal
            self.app.pop_screen()
        elif event.button.id == "cancel-delete":
            # Close modal without deleting
            self.app.pop_screen()


class ChatWindow(App):
    CSS = """
    Screen {
        layout: horizontal;
    }

    Button {
        width: 100%;
    }

    .sidebar {
        width: 20%;
        min-width: 40;
        height: 100%;
        background: $background;
    }

    .search {
        margin: 1;
    }

    .history {
        width: 100%;
    }

    .conversation-container {
        width: 100%;
        height: auto;
        layout: horizontal;
    }

    .conversation {
        width: 85%;
        text-align: left;
    }

    .delete-conversation {
        width: 18%;
        height: 100%;
        min-width: 8;
        color: $text-muted;
    }

    .delete-conversation:hover {
        color: $error;
    }

    .chat {
        max-width: 80%;
        height: 100%;
    }

    .chat-log {
        height: 1fr;
        background: $surface;
    }

    .chat-input {
        width: 100%;
        border: round $background;
    }

    .message {
        border: round $boost;
    }

    .message.user {
        margin: 1 0 1 40;
    }

    .message.assistant {
        margin: 1 40 1 0;
    }

    /* Settings Modal */
    SettingsScreen {
        align: center middle;
    }

    #settings-dialog {
        padding: 2;
        width: 50%;
        height: auto;
        background: $surface;
        border: tall $primary;
        border-title-align: center;
    }

    #settings-title {
        text-align: center;
        width: 100%;
        margin-bottom: 1;
        text-style: bold;
    }

    #settings-dialog Input {
        margin-bottom: 1;
    }

    #settings-buttons {
        width: 100%;
        height: auto;
        layout: horizontal;
        margin-top: 1;
    }

    #settings-buttons Button {
        width: 50%;
    }

    /* Delete Confirmation Modal */
    DeleteConfirmationScreen {
        align: center middle;
    }

    #delete-dialog {
        padding: 2;
        width: 50%;
        height: auto;
        background: $surface;
        border: tall $error;
        border-title-align: center;
    }

    #delete-title {
        text-align: center;
        width: 100%;
        margin-bottom: 1;
        text-style: bold;
        color: $error;
    }

    #delete-prompt {
        text-align: center;
        width: 100%;
        margin-bottom: 2;
    }

    #delete-buttons {
        width: 100%;
        height: auto;
        layout: horizontal;
        margin-top: 1;
    }

    #delete-buttons Button {
        width: 50%;
    }

    /* Footer */
    Footer {
        background: $background;
        height: 1;
    }
    """
    theme = "flexoki"
    BINDINGS = [
        Binding("ctrl+s", "show_settings", "Settings"),
        Binding("ctrl+n", "new_conversation", "New Conversation"),
        Binding("ctrl+d", "delete_conversation", "Delete Conversation"),
    ]

    def __init__(self) -> None:
        super().__init__()
        init_db()
        self.conversation = None
        self.client = ollama.Client(host=OLLAMA_HOST)
        self.search_timer = None
        self.settings = SETTINGS.copy()

    def compose(self) -> ComposeResult:
        with Vertical(classes="sidebar"):
            yield Input(placeholder="Search...", classes="search")
            with ScrollableContainer(classes="history") as history:
                self.history = history
            yield Button("New Chat", id="new-conversation")

        with Vertical(classes="chat"):
            with ScrollableContainer(classes="chat-log") as log:
                self.chat_log = log
            if DEBUG:
                yield RichLog(auto_scroll=True)

            yield Input(placeholder="Type a message...", classes="chat-input")

        yield Footer()

    def on_mount(self) -> None:
        summaries = load_conversation_summaries()
        for conversation_id, summary, _ in summaries:
            self.conversation_item(summary, conversation_id)

    def debug(self, msg: typing.Any) -> None:
        if DEBUG:
            self.query_one(RichLog).write(msg)

    def reload_settings(self, new_settings: dict) -> None:
        """Reload application settings."""
        self.settings = new_settings
        self.client = ollama.Client(host=new_settings["OLLAMA_HOST"])

        # Show a notification
        self.bell()

    def action_show_settings(self) -> None:
        """Show the settings modal."""
        self.push_screen(SettingsScreen(self.settings))

    def action_new_conversation(self) -> None:
        self.chat_log.remove_children()
        self.conversation = None

    def action_delete_conversation(self) -> None:
        """Delete the current conversation after confirmation."""
        if self.conversation:
            self.push_screen(
                DeleteConfirmationScreen(self.conversation.id, self.conversation.summary)
            )

    def clear_current_conversation(self) -> None:
        """Clear the current conversation from the UI."""
        self.chat_log.remove_children()
        self.conversation = None

    def refresh_conversation_list(self) -> None:
        """Refresh the conversation list in the sidebar."""
        # Clear existing conversations
        self.history.remove_children()

        # Load and display updated conversations
        summaries = load_conversation_summaries()
        for conversation_id, summary, _ in summaries:
            self.conversation_item(summary, conversation_id)

    def summarize(self, message: str) -> str:
        prompt = f"""Summarize the following chat message into a short, 4-6 word title.
Avoid punctuation and quotes.

Message:
{message}
"""
        response = self.client.chat(
            model=self.settings["WEAK_MODEL"],
            messages=[{"role": Role.USER.value, "content": prompt}],
            think=False,
        )
        return response.message.content.capitalize()

    def conversation_item(
        self, summary: str, conversation_id: str, before: ComposeResult | None = None
    ) -> Container:
        """Create a container with conversation button and delete button."""
        container = Container(classes="conversation-container")
        self.history.mount(container, before=before)
        container.mount(Button(summary, id=conversation_id, classes="conversation"))
        container.mount(Button("✕", id=f"delete-{conversation_id}", classes="delete-conversation"))
        return container

    def add_conversation(self, summary: str, conversation_id: str, *, prepend: bool = True) -> None:
        before = None
        if prepend and self.history.children:
            before = self.history.children[0]
        self.conversation_item(summary, conversation_id, before)

    def clear_conversations(self) -> None:
        self.history.remove_children()

    def add_message(self, message: Message) -> None:
        # remove_children() deletes the children, so we deepcopy the message.
        self.chat_log.mount(Message(message.content, message.role))

    def refresh_history(self, search_term: str = "") -> None:
        """Refresh the conversation history based on search term."""
        seen_ids = set()
        summaries = load_conversation_summaries(search_term)
        # remove_children() doesn't appear to be thread-safe. If used here, adding the new
        # summaries will complain about duplicate id's which would be impossible if they were
        # cleared in time. Moreover, probably bad, jerky UX anyways to not update in-place.
        for child in self.history.children:
            # Extract conversation ID from the child's first button
            if child.children and hasattr(child.children[0], "id"):
                conversation_button_id = child.children[0].id
                if any(
                    conversation_button_id == conversation_id for conversation_id, _, _ in summaries
                ):
                    seen_ids.add(conversation_button_id)
                    continue
            child.remove()
        for conversation_id, summary, _ in summaries:
            if conversation_id in seen_ids:
                continue
            self.add_conversation(summary, conversation_id, prepend=False)

    def on_button_pressed(self, event: Button.Pressed) -> None:
        """Event handler called when a button is pressed."""
        if event.button.id == "new-conversation":
            self.action_new_conversation()
            return

        if event.button.id == "settings-button":
            self.action_show_settings()
            return

        # Handle delete button clicks
        if event.button.id and event.button.id.startswith("delete-"):
            conversation_id = event.button.id[7:]  # Remove "delete-" prefix
            # Find the conversation summary for this ID
            conversation = load_conversation_by_id(conversation_id)
            if conversation:
                self.push_screen(DeleteConfirmationScreen(conversation_id, conversation.summary))
            return

        # Load only the specific conversation that was clicked
        if not event.button.id or (self.conversation and self.conversation.id == event.button.id):
            return

        self.chat_log.remove_children()
        conversation = load_conversation_by_id(event.button.id)
        if conversation:
            self.conversation = conversation
            for m in conversation.messages:
                self.add_message(m)

    def on_input_submitted(self, event: Input.Submitted) -> None:
        if "chat-input" in event.input.classes:
            content = event.value.strip()
            if content:
                message = Message(content, Role.USER)
                self.add_message(message)
                if not self.conversation:
                    summary = self.summarize(content)
                    self.conversation = Conversation(summary)
                    self.add_conversation(self.conversation.summary, self.conversation.id)
                self.conversation.messages.append(message)

                response = self.client.chat(
                    model=self.settings["MAIN_MODEL"],
                    messages=[
                        {
                            "role": message.role.value,
                            "content": message.content,
                        }
                        for message in self.conversation.messages
                    ],
                )
                response = Message(response.message.content, Role.ASSISTANT)
                self.add_message(response)
                self.conversation.messages.append(response)
                save_conversation(self.conversation)
            event.input.value = ""
        elif "search" in event.input.classes:
            # Handle search submission (Enter key)
            search_term = event.value.strip()
            self.refresh_history(search_term)

    def on_input_changed(self, event: Input.Changed) -> None:
        """Handle input changes with debouncing for search."""
        if "search" in event.input.classes:
            # Cancel existing timer if it exists
            if self.search_timer:
                self.search_timer.stop()
                self.search_timer = None
            # Set up a new timer for debouncing (0.3 seconds)
            self.search_timer = self.set_timer(
                0.3, lambda: self.refresh_history(event.value.strip())
            )


if __name__ == "__main__":
    ChatWindow().run()
