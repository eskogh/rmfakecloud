import React, { useState } from "react";
import { useHistory } from "react-router-dom";
import { Button, Form } from "react-bootstrap";

import { useAuthState } from "../../common/useAuthContext";
import { loginUser } from "../../common/actions";

import styles from "./Login.module.scss";

const Login = () => {
  let history = useHistory();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");

  const { state, dispatch } = useAuthState(); //read the values of loading and errorMessage from context
  const { errorMessage, loading } = state;

  const handleLogin = async (e) => {
    e.preventDefault();

    let payload = { email: username, password };
    try {
      await loginUser(dispatch, payload);
      history.push("/documents"); //TODO: usenavigate or return redirect
    } catch (error) {
      console.log(error);
    }
  };

  return (
    <div className={styles.container}>
      <section
        className={styles.identity}
        aria-label="rmfakecloud — Your notes. Your cloud. Yours."
      >
        <img
          className="brand-logo-dark"
          src="/assets/brand/logo-dark.png"
          alt="rmfakecloud — Your notes. Your cloud. Yours."
          width="1672"
          height="941"
        />
        <img
          className="brand-logo-light"
          src="/assets/brand/logo-light.png"
          alt="rmfakecloud — Your notes. Your cloud. Yours."
          width="1672"
          height="941"
        />
      </section>
      <div className={styles.formContainer}>
        <p className="eyebrow">YOUR PERSONAL CLOUD</p>
        <h1>Welcome home.</h1>
        <p className={styles.intro}>
          Sign in to your notes, notebooks, and next big ideas.
        </p>
        {errorMessage ? (
          <p role="alert" className={styles.error}>
            {errorMessage}
          </p>
        ) : null}

        <Form onSubmit={handleLogin}>
          <Form.Group className="mb-3">
            <Form.Label htmlFor="username">Username</Form.Label>
            <Form.Control
              id="username"
              value={username}
              autoFocus
              onChange={(e) => setUsername(e.target.value)}
              disabled={loading}
              placeholder="Username"
              autoComplete="username"
            />
          </Form.Group>

          <Form.Group className="mb-3">
            <Form.Label htmlFor="password">Password</Form.Label>
            <Form.Control
              type="password"
              id="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              disabled={loading}
              placeholder="Password"
              autoComplete="current-password"
            />
          </Form.Group>

          <Button type="submit" disabled={loading}>
            {loading ? "Signing in…" : "Sign in"}
          </Button>
        </Form>
      </div>
    </div>
  );
};

export default Login;
