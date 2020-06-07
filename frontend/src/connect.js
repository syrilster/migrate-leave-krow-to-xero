import React, { Component } from "react";
import "./App.css";

const xeroAuthURL = process.env.REACT_APP_XERO_AUTH_URL
const clientID = "EB0E514535E344B1B4C6C62429FD89B9"
const xeroRedirectURI = process.env.REACT_APP_XERO_REDIRECT_URI
const scopes = "offline_access openid payroll.employees payroll.settings"
const randNumber = "116780"

class Connect extends Component {

  render() {
    return (
      <a href={xeroAuthURL + '?response_type=code&client_id=' + clientID + '&redirect_uri=' + xeroRedirectURI + '&scope=' + scopes + '&state=' + randNumber}>
        <img
          src="https://developer.xero.com/static/images/documentation/connect_xero_button_blue_2x.png"
          alt="ConnectToXero"/>
      </a>
    );
  }
}

export default Connect;
