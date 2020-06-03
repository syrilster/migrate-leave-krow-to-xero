import React, { Component } from "react";
import "./App.css";
import Connect from "./connect";
import { Route } from "react-router";
import Upload from "./upload";
import ErrorPage from "./error";
import { BrowserRouter } from "react-router-dom";

class App extends Component {
  render() {
    return (
      <div>
        <BrowserRouter>
          <Route exact path="/">
            <Connect />
          </Route>
          <Route exact path="/upload">
            <Upload />
          </Route>
          <Route exact path="/error">
            <ErrorPage />
          </Route>
        </BrowserRouter>
      </div>
    );
  }
}

export default App;
