//! Shell-style matching of root-level entry names. Names never contain a
//! separator, so `*` and `?` have no path semantics to worry about.

enum Token {
    Literal(char),
    AnyChar,
    Star,
    Class {
        negated: bool,
        ranges: Vec<(char, char)>,
    },
}

impl Token {
    fn accepts(&self, c: char) -> bool {
        match self {
            Token::Literal(literal) => *literal == c,
            Token::AnyChar => true,
            Token::Star => false,
            Token::Class { negated, ranges } => {
                ranges.iter().any(|(lo, hi)| (*lo..=*hi).contains(&c)) != *negated
            }
        }
    }
}

/// A compiled pattern supporting `*`, `?`, `[a-z]`, `[^a-z]`, and `\` escapes.
pub struct Glob(Vec<Token>);

impl Glob {
    /// Compiles `pattern`, returning `None` if it is malformed.
    pub fn new(pattern: &str) -> Option<Glob> {
        let mut chars = pattern.chars().peekable();
        let mut tokens = Vec::new();
        while let Some(c) = chars.next() {
            tokens.push(match c {
                '*' => Token::Star,
                '?' => Token::AnyChar,
                '\\' => Token::Literal(chars.next()?),
                '[' => parse_class(&mut chars)?,
                c => Token::Literal(c),
            });
        }
        Some(Glob(tokens))
    }

    pub fn matches(&self, name: &str) -> bool {
        let tokens = &self.0;
        let name: Vec<char> = name.chars().collect();
        let (mut t, mut n) = (0, 0);
        // Where to resume after a mismatch: the token following the latest
        // star, and the name position that star has consumed up to. Only the
        // latest star ever needs revisiting, so this can't blow up.
        let mut resume = None;
        while n < name.len() {
            match tokens.get(t) {
                Some(Token::Star) => {
                    t += 1;
                    resume = Some((t, n));
                    continue;
                }
                Some(token) if token.accepts(name[n]) => {
                    t += 1;
                    n += 1;
                    continue;
                }
                _ => {}
            }
            let Some((after_star, consumed)) = resume else {
                return false;
            };
            t = after_star;
            n = consumed + 1;
            resume = Some((after_star, n));
        }
        tokens[t..].iter().all(|token| matches!(token, Token::Star))
    }
}

fn parse_class(chars: &mut std::iter::Peekable<std::str::Chars<'_>>) -> Option<Token> {
    let negated = chars.next_if_eq(&'^').is_some();
    let mut ranges = Vec::new();
    loop {
        if !ranges.is_empty() && chars.next_if_eq(&']').is_some() {
            return Some(Token::Class { negated, ranges });
        }
        let lo = class_char(chars)?;
        let hi = if chars.next_if_eq(&'-').is_some() {
            class_char(chars)?
        } else {
            lo
        };
        ranges.push((lo, hi));
    }
}

/// Reads one class member. A bare `-` or `]` here is malformed (so `[]` and
/// `[a-]` are rejected); escape them to match them literally.
fn class_char(chars: &mut std::iter::Peekable<std::str::Chars<'_>>) -> Option<char> {
    match chars.next()? {
        '-' | ']' => None,
        '\\' => chars.next(),
        c => Some(c),
    }
}

#[cfg(test)]
mod tests {
    use super::Glob;

    fn matches(pattern: &str, name: &str) -> bool {
        Glob::new(pattern).unwrap().matches(name)
    }

    #[test]
    fn literals_match_exactly() {
        assert!(matches(".env", ".env"));
        assert!(!matches(".env", ".envrc"));
        assert!(!matches(".envrc", ".env"));
        assert!(matches("", ""));
        assert!(!matches("", "x"));
    }

    #[test]
    fn star_matches_any_run() {
        assert!(matches(".cursor*", ".cursor"));
        assert!(matches(".cursor*", ".cursorignore"));
        assert!(matches("*", ""));
        assert!(matches("*rc", ".npmrc"));
        assert!(matches(".*r*c", ".npmrc"));
        assert!(!matches("*rc", ".npmrc.bak"));
        assert!(matches("**a", "bbba"));
    }

    #[test]
    fn star_backtracks() {
        assert!(matches("*ab", "aab"));
        assert!(matches("a*b*c", "axbxbxc"));
        assert!(!matches("a*b*c", "axbxbx"));
    }

    #[test]
    fn many_stars_stay_fast() {
        let name = "a".repeat(10_000);
        assert!(!matches("*a*a*a*a*a*a*a*a*a*a*a*a*b", &name));
    }

    #[test]
    fn question_mark_matches_one_char() {
        assert!(matches(".?nv", ".env"));
        assert!(!matches(".?nv", ".nv"));
        assert!(matches("?", "é"));
    }

    #[test]
    fn classes() {
        assert!(matches("[ab]c", "bc"));
        assert!(!matches("[ab]c", "cc"));
        assert!(matches("[a-c]", "b"));
        assert!(!matches("[a-c]", "d"));
        assert!(matches("[^a-c]", "d"));
        assert!(!matches("[^a-c]", "b"));
        assert!(matches("[a-cx]", "x"));
    }

    #[test]
    fn escapes() {
        assert!(matches(r"\*", "*"));
        assert!(!matches(r"\*", "x"));
        assert!(matches(r"[\]]", "]"));
        assert!(matches(r"[\-]", "-"));
    }

    #[test]
    fn malformed_patterns_are_rejected() {
        for pattern in ["[", "[a", "[]", "[^]", "[]a]", "[a-]", "[a-", r"\", r"[\"] {
            assert!(
                Glob::new(pattern).is_none(),
                "{pattern} should be malformed"
            );
        }
    }
}
