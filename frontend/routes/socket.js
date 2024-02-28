

module.exports = function(io, runToSocketMap={}) {
    io.on('connection', (socket) => {
        console.log('a user connected');
        socket.on('disconnect', () => {
            console.log('user disconnected');
        });
        socket.on('run', (msg) => {
            console.log('run: ' + JSON.stringify(msg));
        });
    });
}
