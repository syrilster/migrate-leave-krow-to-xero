import React, { Component } from 'react';
import './App.css';
import axios from 'axios'
import { ToastContainer, toast } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { loadProgressBar } from 'axios-progress-bar'


const url = 'http://localhost:8000/v1/migrateLeaves';

class Upload extends Component {

  constructor(props) {
    super(props);
    loadProgressBar();
    this.state = {
      selectedFile: ''
    }
  }

  onChangeHandler = event => {
    console.log(event.target.files[0])
    this.setState({
      selectedFile: event.target.files[0]
    });
  };

  onClickHandler = event => {
    if (this.state.selectedFile === '') {
      toast.error('Please select a file to proceed !!');
      return  
    }

    let formData = new FormData();
    formData.append("file", this.state.selectedFile);
    let config = {
      headers: {
        'Content-Type': 'multipart/form-data',
        'Access-Control-Allow-Origin':'*'
      }
    };

    axios
      .post(url, formData, config)
      .then(res => {
        console.log("Status: ", res.status);
        toast.success("Leaves Processed Successfully :)");
      })
      .catch(err => {
        console.log("Error: ", err.message);
        toast.error("There was a server error!!");
      });
  };

  
  render() {
    return (
      <div class="container">
        <div class="row">
          <div class="offset-md-3 col-md-6">
               <div class="form-group files">
                <label>Upload Leave Extract From Krow </label>
                <input type="file" class="form-control" onChange={this.onChangeHandler}/>
              </div>  
              <div class="form-group">
                <ToastContainer />
              </div> 
              <button type="button" class="btn btn-success btn-block" onClick={this.onClickHandler}>Upload</button>
        </div>
      </div>
      </div>
    );
  }
  }


export default Upload;