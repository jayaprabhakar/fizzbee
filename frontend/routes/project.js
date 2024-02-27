var express = require('express');
var router = express.Router();

/* GET users listing. */
router.get('/', function(req, res, next) {
  res.render('project', { title: 'FizzBee Model Spec' });
});

module.exports = router;
